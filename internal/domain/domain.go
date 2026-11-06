// Package domain define os tipos trocados entre os serviços pelo Kafka.
// Valores monetários são inteiros (centavos) para evitar erro de ponto flutuante.
package domain

import (
	"errors"
	"regexp"
)

// QuoteAsset é a moeda em que os preços são cotados.
const QuoteAsset = "BRL"

// Tópicos do Kafka. Ambos usam o código do ativo como chave de partição.
const (
	TopicOrders = "orders"
	TopicTrades = "trades"
)

type Side string

const (
	Buy  Side = "BUY"
	Sell Side = "SELL"
)

// Order é o comando publicado no tópico "orders" (chave = Symbol).
// Apenas ordens limitadas existem nesta versão.
type Order struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Symbol    string `json:"symbol"`
	Side      Side   `json:"side"`
	Price     int64  `json:"price"` // centavos por unidade
	Qty       int64  `json:"qty"`   // unidades inteiras do ativo
	CreatedAt int64  `json:"created_at"`
}

// Trade é publicado no tópico "trades" (chave = Symbol).
// ID e Seq são determinísticos: reprocessar o mesmo log gera os mesmos valores.
type Trade struct {
	ID          string `json:"id"`
	Symbol      string `json:"symbol"`
	Seq         int64  `json:"seq"` // sequência por ativo, sem lacunas
	Price       int64  `json:"price"`
	Qty         int64  `json:"qty"`
	BuyOrderID  string `json:"buy_order_id"`
	SellOrderID string `json:"sell_order_id"`
	BuyerID     string `json:"buyer_id"`
	SellerID    string `json:"seller_id"`
	TakerSide   Side   `json:"taker_side"`
	ExecutedAt  int64  `json:"executed_at"` // timestamp do registro da ordem agressora
}

var symbolRe = regexp.MustCompile(`^[A-Z0-9]{2,10}$`)

var (
	ErrSymbol = errors.New("symbol deve ter de 2 a 10 caracteres A-Z/0-9")
	ErrSide   = errors.New("side deve ser BUY ou SELL")
	ErrPrice  = errors.New("price deve ser maior que zero")
	ErrQty    = errors.New("qty deve ser maior que zero")
	ErrSize   = errors.New("price * qty excede o limite permitido")
)

// MaxNotional limita price*qty para impedir overflow de int64.
const MaxNotional int64 = 1 << 50

// Validate confere as regras que valem em qualquer serviço.
func (o Order) Validate() error {
	switch {
	case !symbolRe.MatchString(o.Symbol):
		return ErrSymbol
	case o.Side != Buy && o.Side != Sell:
		return ErrSide
	case o.Price <= 0:
		return ErrPrice
	case o.Qty <= 0:
		return ErrQty
	case o.Price > MaxNotional/o.Qty:
		return ErrSize
	}
	return nil
}

// Reservation devolve o ativo e o valor que precisam ficar bloqueados na carteira.
// Compra bloqueia BRL (price*qty); venda bloqueia o próprio ativo (qty).
func (o Order) Reservation() (asset string, amount int64) {
	if o.Side == Buy {
		return QuoteAsset, o.Price * o.Qty
	}
	return o.Symbol, o.Qty
}
