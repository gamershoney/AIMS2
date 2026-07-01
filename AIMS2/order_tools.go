package main

type InspectVendor struct{}

func (t *InspectVendor) Name() string {
	return "inspect_vendor"
}

func (t *InspectVendor) Description() string {
	return "Use this tool when needing to inspect items within a vendor. Will have a sub-agent review a specific vendor and return a drafted order from that vendor."
}

func (t *InspectVendor) Parameters() Parameters {
	return Parameters{
		Type: "object",
		Properties: map[string]Property{
			"vendor_number":     {Type: "string", Description: "Vendor number you would like to return the items from."},
			"include_critical":  {Type: "bool", Description: "Enter true to include critical items. False to exclude"},
			"include_low_stock": {Type: "boolean", Description: "Enter true to include items with less than a months usage."},
			"limit":             {Type: "number", Description: "Enter the maximum results you would like to return."},
		},
		Required: []string{"vendor_number"},
	}
}
