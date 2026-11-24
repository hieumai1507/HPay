package wallet

// DefaultCurrency is the only currency of the product: US dollars, amounts in integer cents
// (ADR-0019). It is the one place in Go that spells the code; the database default and the three
// system wallets say the same in migration 000002.
const DefaultCurrency = "USD"
