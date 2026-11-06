package orderbook

import (
	"reflect"
	"testing"

	"github.com/equipe-sd/exchange/internal/domain"
)

func order(id, user string, side domain.Side, price, qty int64) domain.Order {
	return domain.Order{ID: id, UserID: user, Symbol: "BTC", Side: side, Price: price, Qty: qty}
}

func TestOrdersThatDoNotCrossRestInTheBook(t *testing.T) {
	b := New("BTC")
	if tr := b.Submit(order("b1", "ana", domain.Buy, 100, 5), "x1", 0); len(tr) != 0 {
		t.Fatalf("esperava 0 negociações, veio %d", len(tr))
	}
	if tr := b.Submit(order("s1", "bia", domain.Sell, 101, 5), "x2", 0); len(tr) != 0 {
		t.Fatalf("esperava 0 negociações, veio %d", len(tr))
	}
	bids, asks := b.Depth(5)
	if !reflect.DeepEqual(bids, []Level{{100, 5}}) || !reflect.DeepEqual(asks, []Level{{101, 5}}) {
		t.Fatalf("livro inesperado: bids=%v asks=%v", bids, asks)
	}
}

func TestTradeHappensAtMakerPrice(t *testing.T) {
	b := New("BTC")
	b.Submit(order("s1", "bia", domain.Sell, 100, 5), "x1", 0)
	tr := b.Submit(order("b1", "ana", domain.Buy, 105, 5), "x2", 0)
	if len(tr) != 1 || tr[0].Price != 100 || tr[0].Qty != 5 {
		t.Fatalf("negociação inesperada: %+v", tr)
	}
	if tr[0].BuyerID != "ana" || tr[0].SellerID != "bia" || tr[0].TakerSide != domain.Buy {
		t.Fatalf("lados trocados: %+v", tr[0])
	}
}

func TestPriceTimePriority(t *testing.T) {
	b := New("BTC")
	b.Submit(order("s1", "bia", domain.Sell, 101, 3), "x1", 0)  // pior preço
	b.Submit(order("s2", "caio", domain.Sell, 100, 2), "x2", 0) // melhor preço, chegou antes
	b.Submit(order("s3", "duda", domain.Sell, 100, 2), "x3", 0) // mesmo preço, chegou depois

	tr := b.Submit(order("b1", "ana", domain.Buy, 101, 5), "x4", 0)
	got := []string{}
	for _, x := range tr {
		got = append(got, x.SellOrderID)
	}
	if want := []string{"s2", "s3", "s1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ordem de execução %v, esperado %v", got, want)
	}
	_, asks := b.Depth(5)
	if !reflect.DeepEqual(asks, []Level{{101, 2}}) {
		t.Fatalf("sobra inesperada: %v", asks)
	}
}

func TestPartialFillLeavesRemainderOfTaker(t *testing.T) {
	b := New("BTC")
	b.Submit(order("s1", "bia", domain.Sell, 100, 2), "x1", 0)
	tr := b.Submit(order("b1", "ana", domain.Buy, 100, 5), "x2", 0)
	if len(tr) != 1 || tr[0].Qty != 2 {
		t.Fatalf("esperava execução parcial de 2, veio %+v", tr)
	}
	bids, asks := b.Depth(5)
	if !reflect.DeepEqual(bids, []Level{{100, 3}}) || len(asks) != 0 {
		t.Fatalf("livro inesperado: bids=%v asks=%v", bids, asks)
	}
}

func TestSequenceAndIDsAreDeterministic(t *testing.T) {
	run := func() []domain.Trade {
		b := New("BTC")
		var all []domain.Trade
		inputs := []domain.Order{
			order("s1", "bia", domain.Sell, 100, 1),
			order("s2", "bia", domain.Sell, 100, 1),
			order("b1", "ana", domain.Buy, 100, 2),
			order("b2", "ana", domain.Buy, 99, 1),
			order("s3", "caio", domain.Sell, 98, 1),
		}
		for i, o := range inputs {
			all = append(all, b.Submit(o, "p0-o"+string(rune('0'+i)), int64(i))...)
		}
		return all
	}
	a, c := run(), run()
	if !reflect.DeepEqual(a, c) {
		t.Fatal("mesma entrada gerou saídas diferentes")
	}
	for i, tr := range a {
		if tr.Seq != int64(i+1) {
			t.Fatalf("seq com lacuna: posição %d tem seq %d", i, tr.Seq)
		}
	}
}

func FuzzBookNeverCrosses(f *testing.F) {
	f.Add([]byte{1, 100, 5, 0, 101, 3, 0, 99, 2})
	f.Fuzz(func(t *testing.T, data []byte) {
		b := New("BTC")
		for i := 0; i+2 < len(data); i += 3 {
			side := domain.Buy
			if data[i]%2 == 0 {
				side = domain.Sell
			}
			b.Submit(order("o", "u", side, int64(data[i+1])+1, int64(data[i+2]%10)+1), "x", 0)
			bids, asks := b.Depth(1)
			if len(bids) == 1 && len(asks) == 1 && bids[0].Price >= asks[0].Price {
				t.Fatalf("livro cruzado: bid %d >= ask %d", bids[0].Price, asks[0].Price)
			}
		}
	})
}
