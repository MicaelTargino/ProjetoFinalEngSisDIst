// Package orderbook implementa um livro de ofertas com prioridade preço-tempo.
// O livro não tem concorrência interna: cada ativo é processado por uma única
// goroutine (ver ADR-001), o que torna o resultado determinístico.
package orderbook

import (
	"fmt"
	"sort"

	"github.com/equipe-sd/exchange/internal/domain"
)

type entry struct {
	order     domain.Order
	remaining int64
}

type level struct {
	price int64
	queue []*entry // FIFO: a ordem mais antiga é executada primeiro
}

// Book é o livro de um único ativo.
type Book struct {
	symbol string
	bids   []*level // ordenado do maior para o menor preço
	asks   []*level // ordenado do menor para o maior preço
	seq    int64    // sequência de negociações do ativo
}

func New(symbol string) *Book { return &Book{symbol: symbol} }

// Seq devolve o número da última negociação executada.
func (b *Book) Seq() int64 { return b.seq }

// Submit casa uma ordem limitada contra o lado oposto e deixa o restante no livro.
// idPrefix identifica a ordem agressora de forma determinística (ex.: "p3-o1042"),
// e cada negociação recebe o ID "<idPrefix>-<n>".
func (b *Book) Submit(o domain.Order, idPrefix string, ts int64) []domain.Trade {
	opposite := &b.asks
	if o.Side == domain.Sell {
		opposite = &b.bids
	}

	remaining := o.Qty
	var trades []domain.Trade
	for remaining > 0 && len(*opposite) > 0 {
		best := (*opposite)[0]
		if !crosses(o, best.price) {
			break
		}
		for remaining > 0 && len(best.queue) > 0 {
			maker := best.queue[0]
			qty := min(remaining, maker.remaining)
			remaining -= qty
			maker.remaining -= qty
			b.seq++
			trades = append(trades, b.trade(o, maker.order, best.price, qty, fmt.Sprintf("%s-%d", idPrefix, len(trades)), ts))
			if maker.remaining == 0 {
				best.queue[0] = nil
				best.queue = best.queue[1:]
			}
		}
		if len(best.queue) == 0 {
			*opposite = (*opposite)[1:]
		}
	}
	if remaining > 0 {
		b.rest(o, remaining)
	}
	return trades
}

// A negociação acontece sempre no preço da ordem que já estava no livro (maker).
func (b *Book) trade(taker, maker domain.Order, price, qty int64, id string, ts int64) domain.Trade {
	t := domain.Trade{
		ID: id, Symbol: b.symbol, Seq: b.seq, Price: price, Qty: qty,
		TakerSide: taker.Side, ExecutedAt: ts,
	}
	buy, sell := taker, maker
	if taker.Side == domain.Sell {
		buy, sell = maker, taker
	}
	t.BuyOrderID, t.BuyerID = buy.ID, buy.UserID
	t.SellOrderID, t.SellerID = sell.ID, sell.UserID
	return t
}

func crosses(o domain.Order, bestOpposite int64) bool {
	if o.Side == domain.Buy {
		return o.Price >= bestOpposite
	}
	return o.Price <= bestOpposite
}

func (b *Book) rest(o domain.Order, remaining int64) {
	side := &b.bids
	// before(p) diz se o nível de preço p vem antes do novo preço na fila.
	before := func(p int64) bool { return p > o.Price }
	if o.Side == domain.Sell {
		side = &b.asks
		before = func(p int64) bool { return p < o.Price }
	}
	levels := *side
	i := sort.Search(len(levels), func(i int) bool { return !before(levels[i].price) })
	e := &entry{order: o, remaining: remaining}
	if i < len(levels) && levels[i].price == o.Price {
		levels[i].queue = append(levels[i].queue, e)
		return
	}
	levels = append(levels, nil)
	copy(levels[i+1:], levels[i:])
	levels[i] = &level{price: o.Price, queue: []*entry{e}}
	*side = levels
}

// Level resume um nível de preço para consulta.
type Level struct {
	Price int64 `json:"price"`
	Qty   int64 `json:"qty"`
}

// Depth devolve até n níveis de cada lado do livro.
func (b *Book) Depth(n int) (bids, asks []Level) {
	return summarize(b.bids, n), summarize(b.asks, n)
}

func summarize(levels []*level, n int) []Level {
	out := make([]Level, 0, min(n, len(levels)))
	for _, l := range levels[:min(n, len(levels))] {
		var qty int64
		for _, e := range l.queue {
			qty += e.remaining
		}
		out = append(out, Level{Price: l.price, Qty: qty})
	}
	return out
}
