package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	var cep string

	flag.StringVar(&cep, "cep", "", "defines which 'cep' code to consult. must not contain special characters")
	flag.Parse()

	if cep == "" {
		log.Println("cep can not be empty")
		return
	}

	if len(cep) != 8 {
		log.Println("cep must have eight digits")
		return
	}

	if _, err := strconv.Atoi(cep); err != nil {
		log.Println("cep must contain only digits")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var (
		err        error
		resultChan = make(chan *cepInfo)
	)

	go func() {
		resp, err := requestCepFromBrasilApi(ctx, cep)
		if err != nil {
			log.Println("error threading brasil-api:", err)
			return
		}
		resultChan <- resp
	}()

	go func() {
		resp, err := requestCepFromViaCep(ctx, cep)
		if err != nil {
			log.Println("error threading via-cep:", err)
			return
		}
		resultChan <- resp
	}()

	select {
	case info := <-resultChan:
		cancel()
		if info.Cep == "" {
			err = errors.New("cep not found")
		} else {
			err = json.NewEncoder(os.Stdout).Encode(info)
		}
	case <-ctx.Done():
		err = ctx.Err()
	}

	if err != nil {
		log.Println(err)
	}
}

type cepInfo struct {
	Cep          string `json:"cep"`
	State        string `json:"state"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	Street       string `json:"street"`
	Api          string `json:"api"`
}

func requestCepFromBrasilApi(ctx context.Context, cep string) (*cepInfo, error) {
	type respData struct {
		Cep          string `json:"cep"`
		State        string `json:"state"`
		City         string `json:"city"`
		Neighborhood string `json:"neighborhood"`
		Street       string `json:"street"`
		Service      string `json:"service"`
	}

	var data respData

	err := request(ctx, fmt.Sprintf("https://brasilapi.com.br/api/cep/v1/%s", cep), &data)
	if err != nil {
		return nil, fmt.Errorf("error requesting brasil api: %w", err)
	}

	resp := &cepInfo{
		Cep:          data.Cep,
		State:        data.State,
		City:         data.City,
		Neighborhood: data.Neighborhood,
		Street:       data.Street,
		Api:          "brasil-api",
	}

	return resp, err

}

func requestCepFromViaCep(ctx context.Context, cep string) (*cepInfo, error) {
	type respData struct {
		Cep         string `json:"cep"`
		Logradouro  string `json:"logradouro"`
		Complemento string `json:"complemento"`
		Unidade     string `json:"unidade"`
		Bairro      string `json:"bairro"`
		Localidade  string `json:"localidade"`
		Uf          string `json:"uf"`
		Ibge        string `json:"ibge"`
		Gia         string `json:"gia"`
		Ddd         string `json:"ddd"`
		Siafi       string `json:"siafi"`
	}

	var data respData

	err := request(ctx, fmt.Sprintf("http://viacep.com.br/ws/%s/json/", cep), &data)
	if err != nil {
		return nil, fmt.Errorf("error requesting brasil api: %w", err)
	}

	resp := &cepInfo{
		Cep:          data.Cep,
		State:        data.Uf,
		City:         data.Localidade,
		Neighborhood: data.Bairro,
		Street:       data.Logradouro,
		Api:          "via-cep",
	}

	return resp, err
}

func request(ctx context.Context, url string, data any) error {

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("error to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error to do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error to read body: %w", err)
	}

	err = json.Unmarshal(body, data)
	if err != nil {
		return fmt.Errorf("error to Unmarshal body: %w", err)
	}

	return nil
}
