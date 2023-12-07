package entities

import (
	"container/heap"
	"sync"
)

type Book struct {
	Order           []*Order
	Transactions    []*Transaction
	OrdersChannel   chan *Order
	OrderChannelOut chan *Order
	Wg              *sync.WaitGroup
}

func NewBook(orderChannel chan *Order, orderChannelOut chan *Order, wg *sync.WaitGroup) *Book {
	return &Book{
		Order:           []*Order{},
		Transactions:    []*Transaction{},
		OrdersChannel:   orderChannel,
		OrderChannelOut: orderChannelOut,
		Wg:              wg,
	}
}

func (b *Book) Trade() {
	buyOrders := make(map[string]*OrderQueue)
	sellOrders := make(map[string]*OrderQueue)
	// buyOrders := NewOrderQueue()
	// sellOrders := NewOrderQueue()

	// heap.Init(buyOrders)
	// heap.Init(sellOrders)

	for order := range b.OrdersChannel {
		asset := order.Asset.ID

		if buyOrders[asset] == nil {
			buyOrders[asset] = NewOrderQueue()
			heap.Init(buyOrders[asset])
		}

		if sellOrders[asset] == nil {
			sellOrders[asset] = NewOrderQueue()
			heap.Init(sellOrders[asset])
		}
		if order.OrderType == "BUY" {
			buyOrders[asset].Push(order)
			if sellOrders[asset].Len() > 0 && sellOrders[asset].Orders[0].Price <= order.Price {
				sellOrder := sellOrders[asset].Pop().(*Order)
				if sellOrder.PendingShares > 0 {
					transaction := NewTransaction(sellOrder, order, order.Shares, sellOrder.Price)
					b.AddTransaction(transaction, b.Wg)
					sellOrder.Transactions = append(sellOrder.Transactions, transaction)
					order.Transactions = append(order.Transactions, transaction)
					b.OrderChannelOut <- sellOrder
					b.OrderChannelOut <- order
					if sellOrder.PendingShares > 0 {
						sellOrders[asset].Push(sellOrder)
					}
				}
			}
		} else if order.OrderType == "SELL" {
			sellOrders[asset].Push(order)
			if buyOrders[asset].Len() > 0 && buyOrders[asset].Orders[0].Price >= order.Price {
				buyOrder := buyOrders[asset].Pop().(*Order)
				if buyOrder.PendingShares > 0 {
					transaction := NewTransaction(order, buyOrder, order.Shares, buyOrder.Price)
					b.AddTransaction(transaction, b.Wg)
					buyOrder.Transactions = append(buyOrder.Transactions, transaction)
					order.Transactions = append(order.Transactions, transaction)
					b.OrderChannelOut <- buyOrder
					b.OrderChannelOut <- order
					if buyOrder.PendingShares > 0 {
						buyOrders[asset].Push(buyOrder)
					}
				}
			}

		}
	}
}

func (b *Book) AddTransaction(transaction *Transaction, wg *sync.WaitGroup) {
	defer wg.Done()

	sellingShares := transaction.SellingOrder.PendingShares
	buyingShares := transaction.SellingOrder.PendingShares

	minShares := sellingShares
	if buyingShares < minShares {
		minShares = buyingShares
	}

	transaction.SellingOrder.Investor.UpdateAssetPosition(transaction.SellingOrder.Asset.ID, -minShares)
	// transaction.SellingOrder.PendingShares -= minShares
	transaction.AddSellOrderPendingShares(-minShares)

	transaction.BuyingOrder.Investor.UpdateAssetPosition(transaction.BuyingOrder.Asset.ID, minShares)
	transaction.AddBuyOrderPendingShares(-minShares)
	// transaction.BuyingOrder.PendingShares -= minShares

	transaction.CalculateTotal(transaction.Shares, transaction.BuyingOrder.Price)
	// transaction.Total = float64(transaction.Shares) * transaction.BuyingOrder.Price

	transaction.CloseBuyOrder()
	transaction.CloseSellOrder()

	// if transaction.BuyingOrder.PendingShares == 0 {
	// 	transaction.BuyingOrder.Status = "CLOSED"
	// }
	// if transaction.SellingOrder.PendingShares == 0 {
	// 	transaction.SellingOrder.Status = "CLOSED"
	// }

	b.Transactions = append(b.Transactions, transaction)
}
