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

CREATE TABLE IF NOT EXISTS inventory_notes(
  id INTEGER PRIMARY KEY AUTOINCREMENT,

  stock_number TEXT NOT NULL,
  location REAL NOT NULL DEFAULT '1',

  note TEXT NOT NULL DEFAULT '',
  is_critical BOOLEAN NOT NULL DEFAULT FALSE,

  minimum_on_hand REAL DEFAULT 0,
  target_on_hand REAL DEFAULT 0,

  priority INTEGER NOT NULL DEFAULT 0,

  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT DEFAULT CURRENT_TIMESTAMP,

  UNIQUE(stock_number)
);
