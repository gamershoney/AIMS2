CREATE TABLE IF NOT EXISTS inventory (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  location REAL NOT NULL,
  stock_number TEXT NOT NULL,
  description TEXT,
  vendor_number TEXT,
  vendor_product_number TEXT,
  unit TEXT,
  price REAL,
  cost REAL,
  on_hand REAL,
  avg_usage REAL,
  last_sale TEXT,
  last_receipt TEXT,
  last_order TEXT
);

CREATE INDEX IF NOT EXISTS idx_inventory_stock_number
ON inventory(stock_number);

CREATE INDEX IF NOT EXISTS idx_inventory_description
ON inventory(description);
