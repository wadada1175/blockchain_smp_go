package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcd/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

type Wallet struct {
	privateKey        *ecdsa.PrivateKey
	publicKey         *ecdsa.PublicKey
	blockchianAddress string
}

func NewWallet() *Wallet {
	//step1:ECDSA暗号を使って秘密鍵と公開鍵を生成
	w := new(Wallet)
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	w.privateKey = privateKey
	w.publicKey = &privateKey.PublicKey
	//stpp2:公開鍵をSHA-256でハッシュ化する
	h2 := sha256.New()
	h2.Write(w.publicKey.X.Bytes())
	h2.Write(w.publicKey.Y.Bytes())
	digest2 := h2.Sum(nil)
	//step3:step2のハッシュ値をRIPEMD-160でハッシュ化する
	h3 := ripemd160.New()
	h3.Write(digest2)
	digest3 := h3.Sum(nil)
	//step4: RIPEMD-160ハッシュの前にバージョンバイトを追加（メインネットワークの場合は0x00）
	vd4 := make([]byte, 21)
	vd4[0] = 0x00
	copy(vd4[1:], digest3)
	//step5:拡張RIPEMD-160の結果に対してSHA-256ハッシュを実行する。
	h5 := sha256.New()
	h5.Write(vd4)
	digest5 := h5.Sum(nil)
	//step6:直前のSHA-256ハッシュの結果に対してSHA-256ハッシュを実行する。
	h6 := sha256.New()
	h6.Write(digest5)
	digest6 := h6.Sum(nil)
	//step7:step6のSHA-256ハッシュの最初の4バイトを取る。これがアドレスチェックサムである。
	chsum := digest6[:4]
	//step8:ステージ 4 の拡張 RIPEMD-160 ハッシュの最後に、ステージ 7 の 4 つのチェックサムバイトを追加します。これが25バイトのバイナリBitcoin Addressである。
	dc8 := make([]byte, 25)
	copy(dc8[:21], vd4[:])
	copy(dc8[21:], chsum[:])
	//step9:Base58Check エンコーディングを使用して、バイト文字列から base58 文字列に変換します。これは最も一般的に使用されるビットコインアドレスの形式です。
	address := base58.Encode(dc8)
	w.blockchianAddress = address
	return w
}

func (w *Wallet) PrivateKey() *ecdsa.PrivateKey {
	return w.privateKey
}

func (w *Wallet) PrivateKeyString() string {
	return fmt.Sprintf("%x", w.privateKey.D.Bytes())
}

func (w *Wallet) PublicKey() *ecdsa.PublicKey {
	return w.publicKey
}

func (w *Wallet) PublicKeyString() string {
	return fmt.Sprintf("%064x%064x", w.publicKey.X.Bytes(), w.publicKey.Y.Bytes())
}

func (w *Wallet) BlockchainAddress() string {
	return w.blockchianAddress
}

func (w *Wallet) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PrivateKey        string `json:"private_key"`
		PublicKey         string `json:"public_key"`
		BlockChainAddress string `json:"blockchain_address"`
	}{
		PrivateKey:        w.PrivateKeyString(),
		PublicKey:         w.PublicKeyString(),
		BlockChainAddress: w.BlockchainAddress(),
	})
}
