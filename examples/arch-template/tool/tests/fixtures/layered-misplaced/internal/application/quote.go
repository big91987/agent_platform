package application

import "example.com/fixture/internal/domain"

func QuotePrice(order domain.Order) int64 {
	if order.Premium && order.Total >= 1000 {
		return order.Total * 90 / 100
	}
	return order.Total
}
