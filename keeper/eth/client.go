package eth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cmingxu/wallet-keeper/keeper"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
)

const PASSWORD = "password"

type Client struct {
	l *log.Logger

	// Checkout https://github.com/ethereum/go-ethereum/blob/master/rpc/client.go
	// for more details.
	ethRpcClient *rpc.Client

	// fs directory where to store wallet
	walletDir string

	// keystore
	store *keystore.KeyStore

	// account/address map lock, since ethereum doesn't support account
	// we should have our own account/address map internally.

	// only with this map we can provide services for the upstream services.
	accountPath        string
	accountAddressMap  map[string]string
	accountAddressLock sync.Mutex
}

// TODO move defensive logic
func NewClient(host, walletDir, accountPath, logDir string) (*Client, error) {
	client := &Client{
		walletDir:          walletDir,
		accountPath:        accountPath,
		accountAddressMap:  make(map[string]string),
		accountAddressLock: sync.Mutex{},
	}

	// accountAddressMap initialization
	stat, err := os.Stat(client.accountPath)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, errors.New(fmt.Sprintf("%s is not a valid file", client.accountPath))
	}
	file, err := os.Open(client.accountPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&client.accountAddressMap)
	if err != nil {
		return nil, err
	}

	// keystore initialization
	stat, err = os.Stat(walletDir)
	if err != nil {
		return nil, nil
	}

	if !stat.IsDir() {
		return nil, errors.New(fmt.Sprintf("%s is not a directory", walletDir))
	}
	client.store = keystore.NewKeyStore(walletDir, keystore.StandardScryptN, keystore.StandardScryptP)

	// rpcClient initialization
	rpcClient, err := rpc.Dial(host)
	if err != nil {
		return nil, err
	}
	client.ethRpcClient = rpcClient

	// log initialization
	logPath := filepath.Join(logDir, "eth.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
	if err != nil {
		return nil, err
	}

	client.l = &log.Logger{
		Out:       logFile,
		Formatter: new(log.JSONFormatter),
	}

	return client, nil
}

// Ping
func (client *Client) Ping() error {
	return nil
}

// GetBlockCount
func (client *Client) GetBlockCount() (int64, error) {
	var num string
	err := client.ethRpcClient.CallContext(context.Background(), &num, "eth_blockNumber")
	if err != nil {
		return 0, err
	}

	big, err := hexutil.DecodeBig(num)
	if err != nil {
		return 0, err
	}

	return big.Int64(), nil
}

// GetAddress - default address
func (client *Client) GetAddress(account string) (string, error) {
	address, ok := client.accountAddressMap[account]
	if !ok {
		return "", errors.New(fmt.Sprintf("%s no exists", account))
	}

	return address, nil
}

// Create Account
func (client *Client) CreateAccount(account string) (keeper.Account, error) {
	address, _ := client.GetAddress(account)
	if len(address) > 0 {
		return keeper.Account{}, errors.New(fmt.Sprintf("%s exists", account))
	}

	acc, err := client.store.NewAccount(PASSWORD)
	if err != nil {
		return keeper.Account{}, err
	}

	client.accountAddressLock.Lock()
	client.accountAddressMap[account] = acc.Address.Hex()
	client.accountAddressLock.Unlock()

	// TODO need more robust solution
	client.persistAccountMap()

	return keeper.Account{
		Account: account,
		Balance: 0,
		Addresses: []string{
			acc.Address.Hex(),
		},
	}, nil
}

// GetAccountInfo
func (client *Client) GetAccountInfo(address string, minConf int) (keeper.Account, error) {
	return keeper.Account{}, nil
}

// TODO
// GetNewAddress does map to `getnewaddress` rpc call now
// rpcclient doesn't have such golang wrapper func.
func (client *Client) GetNewAddress(account string) (string, error) {
	return "", errors.New("not valid operation for ethereum")
}

// GetAddressesByAccount
func (client *Client) GetAddressesByAccount(account string) ([]string, error) {
	address, ok := client.accountAddressMap[account]
	if !ok {
		return []string{}, errors.New(fmt.Sprintf("%s not exists", account))
	}

	return []string{address}, nil
}

// ListAccountsMinConf
func (client *Client) ListAccountsMinConf(conf int) (map[string]float64, error) {
	return make(map[string]float64), nil
}

// SendToAddress
func (client *Client) SendToAddress(address string, amount float64) error {
	return nil
}

// TODO check validity of account and have sufficent balance
func (client *Client) SendFrom(account, address string, amount float64) error {
	return nil
}

// ListUnspentMin
func (client *Client) ListUnspentMin(minConf int) ([]btcjson.ListUnspentResult, error) {
	return []btcjson.ListUnspentResult{}, errors.New("ethereum does not support UXTO")
}

// Move
func (client *Client) Move(from, to string, amount float64) (bool, error) {
	return true, nil
}

func (client *Client) persistAccountMap() error {
	file, err := os.Open(client.accountPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(&client.accountAddressMap)
}
