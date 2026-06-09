package importer

type InventoryItem struct {
	ID               int
	StockNumber      string
	Description      string
	VendorNumber     string
	VendorProdNumber string
	Unit             string
	Price            int
	Cost             int
	OnHand           int
	Usage            int
	LastSale         string
	LastReceipt      string
	LastOrder        string
}

var schemaSQL string
