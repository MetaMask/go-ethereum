package eth

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/syndtr/goleveldb/leveldb"
)

// GetSlice response structures

type GetSliceKeysResponse struct {
	SliceID string                      `json:"slice-id"`
	Stem    []string                    `json:"stem"`
	State   [][]string                  `json:"state"`
	Metrics GetSliceKeysResponseMetrics `json:"metrics"`
}

type GetSliceKeysResponseMetrics struct {
	Time map[string]string `json:"time (ms)"` // stem, state, storage (one by one)
}

// GetSliceKeys retrieves a slice from the state, alongside its stem.
//
// Parameters
// - path 		path from root where the slice starts
// - depth		depth to walk from the slice head
// - stateRoot	state root of the GetSliceKeysResponse
func (api *PublicDebugAPI) GetSliceKeys(ctx context.Context, path string, depth int, stateRoot string) (GetSliceKeysResponse, error) {
	var timerStart int64

	// check the given root
	stateRootByte, err := hexutil.Decode(stateRoot)
	if err != nil {
		return GetSliceKeysResponse{},
			fmt.Errorf("incorrect input, expected string representation of hex for root")
	}

	// check the given path
	slicePath := pathStringToKeyBytes(path)
	if slicePath == nil {
		return GetSliceKeysResponse{},
			fmt.Errorf("incorrect input, expected string representation of hex for path")
	}

	// prepare the response object
	response := GetSliceKeysResponse{
		SliceID: fmt.Sprintf("%s-%02d-%s", path, depth, stateRoot[2:8]),
		Stem:    make([]string, 0),
		State:   make([][]string, 0),
		Metrics: GetSliceKeysResponseMetrics{
			Time: make(map[string]string),
		},
	}

	// load a trie with the given state root from the cache (ideally)
	// TODO
	// we want to have the best mechanism to either fetch the trie
	// from the cache geth is using, or well load it and cache it.
	timerStart = time.Now().UnixNano()
	tr, err := api.eth.BlockChain().GetSecureTrie(common.BytesToHash(stateRootByte))
	if err != nil {
		return GetSliceKeysResponse{}, fmt.Errorf("error loading the trie %v", err)
	}

	response.Metrics.Time["00 trie-loading"] = timeDiffToMiliseconds(time.Now().UnixNano() - timerStart)

	// fetch the stem
	timerStart = time.Now().UnixNano()
	it := tr.NewSliceIterator(slicePath)
	it.Next(true)
	// the actual fetching
	stemKeys := it.StemKeys()
	response.Metrics.Time["01 fetch-stem-keys"] = timeDiffToMiliseconds(time.Now().UnixNano() - timerStart)
	// fill the data into the response
	var keyStr string
	for _, key := range stemKeys {
		keyStr = fmt.Sprintf("%x", key)
		response.Stem = append(response.Stem, keyStr)
	}

	// fetch the slice
	timerStart = time.Now().UnixNano()
	it = tr.NewSliceIterator(slicePath)
	stateKeys, _ := it.Slice(depth, false)
	response.Metrics.Time["02 fetch-slice-keys"] = timeDiffToMiliseconds(time.Now().UnixNano() - timerStart)
	// fill the data into the response
	var keys []string
	for _, depthLevel := range stateKeys {
		// remember that we make a separate golang slice per depth level
		if len(depthLevel) == 0 {
			break
		}

		keys = make([]string, 0)
		for _, key := range depthLevel {
			keyStr = fmt.Sprintf("%x", key)
			keys = append(keys, keyStr)
		}
		response.State = append(response.State, keys)
	}

	// fetch the smart contract storage
	// TODO

	// we are done here
	return response, nil
}

func pathStringToKeyBytes(input string) []byte {
	if input == "" {
		return nil
	}

	// first we convert each character to its hex counterpart
	output := make([]byte, 0)
	var b byte
	for _, c := range input {
		switch {
		case '0' <= c && c <= '9':
			b = byte(c - '0')
		case 'a' <= c && c <= 'f':
			b = byte(c - 'a' + 10)
		default:
			return nil
		}

		output = append(output, b)
	}

	return output
}

func timeDiffToMiliseconds(input int64) string {
	return fmt.Sprintf("%.6f", float64(input)/(1000*1000))
}

///////////////////////
//
// We will mutate this later for getTrieNodes([Hash])
//
///////////////////////

// GetLevelDbKey retrieves the value of a key from levelDB backend
func (api *PublicDebugAPI) GetLevelDbKey(ctx context.Context, input string) (string, error) {
	ldb, ok := api.eth.ChainDb().(interface {
		LDB() *leveldb.DB
	})
	if !ok {
		return "", fmt.Errorf("db not found!")
	}

	bb, err := hexutil.Decode(input)
	if err != nil {
		return "", fmt.Errorf("incorrect input, expected string representation of hex")
	}

	value, err := ldb.LDB().Get(bb, nil)
	if err != nil {
		return "", fmt.Errorf("error getting value from levelDB %v", err)
	}

	return fmt.Sprintf("%x", value), nil
}
