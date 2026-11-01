package application

import "example.com/fixture/internal/domain"

func CheckoutPrice(order domain.Order) int64 {
	if order.Premium && order.Total >= 1000 {
		return order.Total - order.Total/10
	}
	return order.Total
}
