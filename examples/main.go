package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/google/uuid"

	ethlib "github.com/w3-developers/go-ethereum-lib"
)

const (
	sepoliaRPCURL = "https://eth-sepolia-testnet.api.pocket.network"
)

func UUIDFromCalldata(calldata string) (uuid.UUID, error) {
	if len(calldata) >= 2 && calldata[:2] == "0x" {
		calldata = calldata[2:]
	}
	data, err := hex.DecodeString(calldata)
	if err != nil {
		return uuid.Nil, err
	}

	if len(data) < 4+32 {
		return uuid.Nil, fmt.Errorf("calldata too short")
	}

	word := data[len(data)-32:]

	var id uuid.UUID
	copy(id[:], word[16:])

	return id, nil
}

func main() {
	contractAddress := "0xC55d61E9c41432eE19Ca0a823A82F1ef15998E58"
	fromPrivateKey := "e230e23c4cd059377fa1d4cea5e83ed95acdf2faa49cca063a59326067199425"
	toAddress := "0x915F48a53E93DFcC973254cAa9c5f252Ccd609Cb"

	nonceManager := ethlib.NewNonceManager()
	client := ethlib.New(
		sepoliaRPCURL,
		"0xcA11bde05977b3631167028862bE2a173976CA11",
		ethlib.WithGasBoost(1.4),
		ethlib.WithNonceManager(nonceManager),
	)

	input, err := client.GetTransactionInput(context.Background(), "0xd213f9332bfa0c47af35c00bd2a27aaf844314d1856cde489703f39d055fcc55")

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(hex.EncodeToString(input))
	fmt.Println(UUIDFromCalldata(hex.EncodeToString(input)))

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
