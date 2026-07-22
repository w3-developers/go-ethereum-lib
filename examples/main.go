package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	ethlib "github.com/w3-developers/go-ethereum-lib"
)

const (
	sepoliaRPCURL = "https://eth-sepolia-testnet.api.pocket.network"
)

func main() {
	contractAddress := "0xC55d61E9c41432eE19Ca0a823A82F1ef15998E58"
	fromPrivateKey := "<>"
	toAddress := "0x915F48a53E93DFcC973254cAa9c5f252Ccd609Cb"

	fromAddress, err := ethlib.PrivateKeyToAddress(fromPrivateKey)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to get address from private key: %w", err))
	}

	client := ethlib.New(
		sepoliaRPCURL,
		"0xcA11bde05977b3631167028862bE2a173976CA11",
		ethlib.WithGasBoost(1.4),
	)

	nonce, err := client.GetNonce(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to get nonce: %w", err))
	}

	txHash1, err := client.TransferToken(
		context.Background(),
		contractAddress,
		toAddress,
		big.NewInt(10000000),
		fromPrivateKey,
		nil,
		big.NewInt(1000000),
		nonce,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(txHash1)

	nonce = nonce.Add(nonce, big.NewInt(1))

	txHash2, err := client.TransferToken(
		context.Background(),
		contractAddress,
		toAddress,
		big.NewInt(10000000),
		fromPrivateKey,
		nil,
		big.NewInt(1000000),
		nonce,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(txHash2)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		err = client.WaitForStatusSuccess(context.Background(), txHash1, 30*time.Second)
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		defer wg.Done()
		err = client.WaitForStatusSuccess(context.Background(), txHash2, 30*time.Second)
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}
