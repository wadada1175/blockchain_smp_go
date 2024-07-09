package block

import (
	"blockchain_smp_go/utils"
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	MiningDifficulty = 3 // マイニング難易度
	MiningSender     = "THE BLOCKCHAIN"
	MiningReward     = 1.0 // マイニング報酬
	MiningTimerSec   = 20  // マイニングタイマー(startmine実行時の間隔)

	BlockchainPortRangeStart      = 5000
	BlockchainPortRangeEnd        = 5004
	NeighborIpRangeStart          = 0
	NeighborIpRangeEnd            = 1
	BlockchainNeighborSyncTimeSec = 20
)

type Blockchain struct {
	transactionPool   []*Transaction
	chain             []*Block
	blockchainAddress string
	port              uint16
	mux               sync.Mutex

	neighbors    []string
	muxNeighbors sync.Mutex
}

// NewBlockchain 新しいブロックチェーンを初期化します。最初のブロックを作成し、チェーンに追加します。
func NewBlockchain(blockchainAddress string, port uint16) *Blockchain {
	block := &Block{}
	blockchain := &Blockchain{
		blockchainAddress: blockchainAddress,
		port:              port,
	}
	blockchain.CreateBlock(0, block.Hash())
	return blockchain
}

func (bc *Blockchain) Chain() []*Block {
	return bc.chain
}

// Run 隣接ノードの同期とチェーンのコンフリクト解決を開始します。
func (bc *Blockchain) Run() {
	bc.StartSyncNeighbors()
	bc.ResolveConflicts()
}

func (bc *Blockchain) SetNeighbors() {
	bc.neighbors = utils.FindNeighbors(
		utils.GetHost(), bc.port,
		NeighborIpRangeStart, NeighborIpRangeEnd,
		BlockchainPortRangeStart, BlockchainPortRangeEnd)
	log.Printf("%v", bc.neighbors)
}

func (bc *Blockchain) SyncNeighbors() {
	bc.muxNeighbors.Lock()
	defer bc.muxNeighbors.Unlock()
	bc.SetNeighbors()
}

func (bc *Blockchain) StartSyncNeighbors() {
	bc.SyncNeighbors()
	time.AfterFunc(time.Second*BlockchainNeighborSyncTimeSec, bc.StartSyncNeighbors)
}

func (bc *Blockchain) CreateBlock(nonce int, previousHash [32]byte) *Block {
	cblock := NewBlock(nonce, previousHash, bc.transactionPool)
	bc.chain = append(bc.chain, cblock)
	bc.transactionPool = []*Transaction{}
	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/transactions", n)
		client := &http.Client{}
		req, err := http.NewRequest("DELETE", endpoint, nil)
		if err != nil {
			log.Printf("ERROR: %v", err)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("ERROR: %v", err)
			continue
		}
		log.Printf("Deleted transactions at %v: %v", n, resp.Status)
	}
	return cblock
}

func (bc *Blockchain) LastBlock() *Block {
	return bc.chain[len(bc.chain)-1]
}

func (bc *Blockchain) CreateTransaction(sender string, recipient string, value float32,
	senderPublicKey *ecdsa.PublicKey, s *utils.Signature) bool {
	isTransacted := bc.AddTransaction(sender, recipient, value, senderPublicKey, s)

	if isTransacted {
		for _, n := range bc.neighbors {
			publicKeyStr := fmt.Sprintf("%064x%064x", senderPublicKey.X.Bytes(), senderPublicKey.Y.Bytes())
			signatureStr := s.String()
			bt := &TransactionRequest{
				SenderBlockchainAddress:    &sender,
				RecipientBlockchainAddress: &recipient,
				SenderPublicKey:            &publicKeyStr,
				Value:                      &value,
				Signature:                  &signatureStr,
			}
			m, _ := json.Marshal(bt)
			buf := bytes.NewBuffer(m)
			endpoint := fmt.Sprintf("http://%s/transactions", n)
			client := &http.Client{}
			req, err := http.NewRequest("PUT", endpoint, buf)
			if err != nil {
				log.Printf("ERROR: %v", err)
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("ERROR: %v", err)
				continue
			}
			log.Printf("Created transaction at %v: %v", n, resp.Status)
		}
	}

	return isTransacted
}

func (bc *Blockchain) AddTransaction(sender string, recipient string, value float32,
	senderPublicKey *ecdsa.PublicKey, s *utils.Signature) bool {
	transaction := NewTransaction(sender, recipient, value)

	if sender == MiningSender {
		log.Println("INFO: Mining reward")
		bc.transactionPool = append(bc.transactionPool, transaction)
		return true
	}

	if bc.VerifyTransactionSignature(senderPublicKey, s, transaction) {

		if bc.CalculateTotalAmount(sender) < value {
			log.Println("ERROR: Not enough balance in a wallet")
			return false
		}

		log.Println("INFO: Transaction signature is valid")
		bc.transactionPool = append(bc.transactionPool, transaction)
		return true
	} else {
		log.Println("ERROR: Verify Transaction")
	}
	return false
}

func (bc *Blockchain) VerifyTransactionSignature(
	senderPublicKey *ecdsa.PublicKey, s *utils.Signature, t *Transaction) bool {
	m, _ := json.Marshal(t)
	h := sha256.Sum256([]byte(m))
	return ecdsa.Verify(senderPublicKey, h[:], s.R, s.S)
}

func (bc *Blockchain) CopyTransactionPool() []*Transaction {
	transactions := make([]*Transaction, 0, len(bc.transactionPool))
	for _, t := range bc.transactionPool {
		transactions = append(transactions,
			NewTransaction(t.SenderBlockchainAddress, t.RecipientBlockchainAddress, t.Value))
	}
	return transactions
}

func (bc *Blockchain) TransactionPool() []*Transaction {
	return bc.transactionPool
}

func (bc *Blockchain) ClearTransactionPool() {
	bc.transactionPool = bc.transactionPool[:0]
}

func (bc *Blockchain) ValidProof(nonce int, previousHash [32]byte, transactions []*Transaction, difficulty int) bool {
	zeros := strings.Repeat("0", difficulty)
	guessBlock := Block{Nonce: nonce, PreviousHash: previousHash, Transactions: transactions}
	guessHashStr := fmt.Sprintf("%x", guessBlock.Hash())
	return guessHashStr[:difficulty] == zeros
}

func (bc *Blockchain) ProofOfWork() int {
	transactions := bc.CopyTransactionPool()
	previousHash := bc.LastBlock().Hash()
	nonce := 0
	for !bc.ValidProof(nonce, previousHash, transactions, MiningDifficulty) {
		nonce++
	}
	return nonce
}

func (bc *Blockchain) Mining() bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	bc.AddTransaction(MiningSender, bc.blockchainAddress, MiningReward, nil, nil)
	nonce := bc.ProofOfWork()
	previousHash := bc.LastBlock().Hash()
	bc.CreateBlock(nonce, previousHash)
	log.Println("blockchain: action=mining, status=success")

	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/consensus", n)
		client := &http.Client{}
		req, err := http.NewRequest("PUT", endpoint, nil)
		if err != nil {
			log.Printf("ERROR: %v", err)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("ERROR: %v", err)
			continue
		}
		log.Printf("Consensus at %v: %v", n, resp.Status)
	}

	return true
}

func (bc *Blockchain) StartMining() {
	bc.Mining()
	time.AfterFunc(MiningTimerSec*time.Second, bc.StartMining)
}

func (bc *Blockchain) CalculateTotalAmount(blockchainAddress string) float32 {
	var totalAmount float32 = 0.0
	for _, b := range bc.chain {
		for _, t := range b.Transactions {
			value := t.Value
			if blockchainAddress == t.RecipientBlockchainAddress {
				totalAmount += value
			}
			if blockchainAddress == t.SenderBlockchainAddress {
				totalAmount -= value
			}
		}
	}
	return totalAmount
}

func (bc *Blockchain) ValidChain(chain []*Block) bool {
	preBlock := chain[0]
	currentIndex := 1
	for currentIndex < len(chain) {
		block := chain[currentIndex]
		if block.PreviousHash != preBlock.Hash() {
			return false
		}

		if !bc.ValidProof(block.Nonce, preBlock.Hash(), block.Transactions, MiningDifficulty) {
			return false
		}

		preBlock = block
		currentIndex++
	}
	return true
}

func (bc *Blockchain) ResolveConflicts() bool {
	var longestChain []*Block = nil
	maxLength := len(bc.chain)

	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/chain", n)
		resp, err := http.Get(endpoint)
		if err != nil {
			log.Printf("ERROR: %v", err)
			continue
		}
		if resp.StatusCode == 200 {
			var bcResp Blockchain
			decoder := json.NewDecoder(resp.Body)
			err = decoder.Decode(&bcResp)
			if err != nil {
				log.Printf("ERROR: %v", err)
				continue
			}

			chain := bcResp.Chain()

			if len(chain) > maxLength && bc.ValidChain(chain) {
				maxLength = len(chain)
				longestChain = chain
			}
		}
	}

	if longestChain != nil {
		bc.chain = longestChain
		log.Printf("blockchain: action=resolve, status=success")
		return true
	}
	log.Printf("blockchain: action=resolve, status=fail")
	return false
}

func (bc *Blockchain) Print() {
	for i, block := range bc.chain {
		fmt.Printf("%s Chain %d %s\n", strings.Repeat("=", 25), i, strings.Repeat("=", 25))
		block.Print()
	}
	fmt.Printf("%s\n", strings.Repeat("*", 25))
}

func (bc *Blockchain) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Blocks []*Block `json:"chain"`
	}{
		Blocks: bc.chain,
	})
}

func (bc *Blockchain) UnmarshalJSON(data []byte) error {
	v := &struct {
		Blocks *[]*Block `json:"chain"`
	}{
		Blocks: &bc.chain,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}
