package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Address struct {
	CEP        string `json:"cep"`
	Logradouro string `json:"logradouro"`
	Bairro     string `json:"bairro"`
	Localidade string `json:"localidade"`
	UF         string `json:"uf"`
}

type BrasilAPIResponse struct {
	CEP          string `json:"cep"`
	State        string `json:"state"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	Street       string `json:"street"`
}

type ViaCEPResponse struct {
	CEP        string `json:"cep"`
	Logradouro string `json:"logradouro"`
	Bairro     string `json:"bairro"`
	Localidade string `json:"localidade"`
	UF         string `json:"uf"`
	Erro       bool   `json:"erro"`
}

type AddressResult struct {
	Address Address
	APIName string
	Error   error
}

func BrasilAPI(cep string, ch chan<- AddressResult, ctx context.Context) {
	url := fmt.Sprintf("https://brasilapi.com.br/api/cep/v1/%s", cep)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		ch <- AddressResult{Error: fmt.Errorf("BrasilAPI: erro ao criar requisição: %w", err)}
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ch <- AddressResult{Error: fmt.Errorf("BrasilAPI: erro na requisição: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ch <- AddressResult{Error: fmt.Errorf("BrasilAPI: status HTTP inesperado: %d", resp.StatusCode)}
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ch <- AddressResult{Error: fmt.Errorf("BrasilAPI: erro ao ler resposta: %w", err)}
		return
	}

	var brasilAPIResp BrasilAPIResponse
	err = json.Unmarshal(body, &brasilAPIResp)
	if err != nil {
		ch <- AddressResult{Error: fmt.Errorf("BrasilAPI: erro ao decodificar JSON: %w", err)}
		return
	}

	address := Address{
		CEP:        brasilAPIResp.CEP,
		Logradouro: brasilAPIResp.Street,
		Bairro:     brasilAPIResp.Neighborhood,
		Localidade: brasilAPIResp.City,
		UF:         brasilAPIResp.State,
	}

	ch <- AddressResult{Address: address, APIName: "BrasilAPI"}
}

func ViaCEP(cep string, ch chan<- AddressResult, ctx context.Context) {
	url := fmt.Sprintf("http://viacep.com.br/ws/%s/json/", cep)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		ch <- AddressResult{Error: fmt.Errorf("ViaCEP: erro ao criar requisição: %w", err)}
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ch <- AddressResult{Error: fmt.Errorf("ViaCEP: erro na requisição: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ch <- AddressResult{Error: fmt.Errorf("ViaCEP: status HTTP inesperado: %d", resp.StatusCode)}
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ch <- AddressResult{Error: fmt.Errorf("ViaCEP: erro ao ler resposta: %w", err)}
		return
	}

	var viaCEPResp ViaCEPResponse
	err = json.Unmarshal(body, &viaCEPResp)
	if err != nil {
		ch <- AddressResult{Error: fmt.Errorf("ViaCEP: erro ao decodificar JSON: %w", err)}
		return
	}

	if viaCEPResp.Erro {
		ch <- AddressResult{Error: fmt.Errorf("ViaCEP: CEP não encontrado ou inválido")}
		return
	}

	address := Address{
		CEP:        viaCEPResp.CEP,
		Logradouro: viaCEPResp.Logradouro,
		Bairro:     viaCEPResp.Bairro,
		Localidade: viaCEPResp.Localidade,
		UF:         viaCEPResp.UF,
	}

	ch <- AddressResult{Address: address, APIName: "ViaCEP"}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Informe o <TEMPO> em milisegundos e <CEP> na chamada do programa")
		os.Exit(1)
	}

	tempo, err := strconv.Atoi(os.Args[1])
	if err != nil {
		tempo = 1
	}

	cep := os.Args[2]

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(tempo)*time.Second)
	defer cancel()

	ch := make(chan AddressResult)

	fmt.Printf("Buscando CEP %s nas APIs (timeout de %ss)...\n", cep, os.Args[1])

	go ViaCEP(cep, ch, ctx)
	go BrasilAPI(cep, ch, ctx)

	select {
	case result := <-ch:
		if result.Error != nil {
			fmt.Printf("Erro ao buscar CEP: %v\n", result.Error)
		} else {
			fmt.Printf("--- Resultado Mais Rápido ---\n")
			fmt.Printf("API: %s\n", result.APIName)
			fmt.Printf("CEP: %s\n", result.Address.CEP)
			fmt.Printf("Logradouro: %s\n", result.Address.Logradouro)
			fmt.Printf("Bairro: %s\n", result.Address.Bairro)
			fmt.Printf("Localidade: %s\n", result.Address.Localidade)
			fmt.Printf("UF: %s\n", result.Address.UF)
		}
	case <-ctx.Done():
		fmt.Printf("Erro: Timeout de %s segundo(s) excedido para o CEP %s. Nenhuma API respondeu a tempo.\n", os.Args[1], cep)
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Println("Motivo do timeout: limite de tempo excedido.")
		} else {
			fmt.Println("Motivo do timeout: contexto cancelado por outro motivo.")
		}
	}
}
