package ethlib

import (
	"context"
	"math/big"
)

type Log struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockHash        string   `json:"blockHash"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
}

func (c *Client) GetLogs(
	ctx context.Context,
	address string,
	fromBlock *big.Int,
	toBlock *big.Int,
	topics []string,
) ([]Log, error) {
	filter := map[string]any{
		"address":   address,
		"fromBlock": BigIntToHex(fromBlock),
		"toBlock":   BigIntToHex(toBlock),
	}

	if len(topics) > 0 {
		filter["topics"] = topics
	}

	var logs []Log
	if err := c.rpcCall(ctx, "eth_getLogs", []any{filter}, &logs); err != nil {
		return nil, err
	}

	return logs, nil
}
