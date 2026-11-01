# Order pricing architecture

Order qualification and discount rules belong to the domain module. Checkout and quote are application entry points for the same pricing policy. A premium customer with a total of at least 1000 receives a ten percent discount. Prices must match between quoting and checkout. The application coordinates use cases; it must not independently own duplicate pricing policies. This is a deliberately small architecture review fixture, not a production order service.
