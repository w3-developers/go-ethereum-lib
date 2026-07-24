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
	fromPrivateKey := ""
	toAddress := "0x915F48a53E93DFcC973254cAa9c5f252Ccd609Cb"

	nonceManager := ethlib.NewNonceManager()
	client := ethlib.New(
		sepoliaRPCURL,
		"0xcA11bde05977b3631167028862bE2a173976CA11",
		ethlib.WithGasBoost(1.4),
		ethlib.WithNonceManager(nonceManager),
	)

	txHash1, err := client.TransferToken(
		context.Background(),
		contractAddress,
		toAddress,
		big.NewInt(10000000),
		fromPrivateKey,
		nil,
		big.NewInt(1000000),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(txHash1)

	txHash2, err := client.TransferToken(
		context.Background(),
		contractAddress,
		toAddress,
		big.NewInt(10000000),
		fromPrivateKey,
		nil,
		big.NewInt(1000000),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(txHash2)

	txHash3, err := client.TransferToken(
		context.Background(),
		contractAddress,
		toAddress,
		big.NewInt(10000000),
		fromPrivateKey,
		nil,
		big.NewInt(1000000),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(txHash3)

	wg := sync.WaitGroup{}
	wg.Add(3)

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

	go func() {
		defer wg.Done()
		err = client.WaitForStatusSuccess(context.Background(), txHash3, 30*time.Second)
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()

	clientNoNonceManager := ethlib.New(
		sepoliaRPCURL,
		"0xcA11bde05977b3631167028862bE2a173976CA11",
		ethlib.WithGasBoost(1.4),
	)

	txHash4, err := clientNoNonceManager.TransferToken(
		context.Background(),
		contractAddress,
		toAddress,
		big.NewInt(10000000),
		fromPrivateKey,
		nil,
		big.NewInt(1000000),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(txHash4)

	err = clientNoNonceManager.WaitForStatusSuccess(context.Background(), txHash4, 30*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("All transactions completed successfully")
}
