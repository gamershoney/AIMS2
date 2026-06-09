CREATE TABLE IF NOT EXISTS inventory (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  stock_number TEXT NOT NULL,
  description TEXT NOT NULL,
  vendor_number TEXT NOT NULL,
  vendor_product_number TEXT,
  unit TEXT,
  price REAL,
  cost REAL,
  on_hand REAL,
  avg_usage REAL,
  last_sale TEXT,
  last_receipt TEXT,
  last_order TEXT,
);

CREATE INDEX IF NOT EXISTS idx_inventory_stock_number
ON inventory(stock_number);

CREATE INDEX IF NOT EXISTS idx_inventory_description
ON inventory(description)
