package extract

import "testing"

// Layouts written in the style of other providers' bills (synthetic data, no real customer):
// none of them has provider-specific code.

func TestTablesGasNorthern(t *testing.T) {
	text := `SUI NORTHERN GAS PIPELINES LIMITED
Consumer ID Billing Month Issue Date Due Date
12345678901 OCT-26 06-Oct-2026 19-Oct-2026
Meter No Previous Reading Present Reading Units (HM3)
G-7781 4512 4530 18
Arrears Current Bill Amount Payable Within Due Date Amount After Due Date
0 2,450 2,450 2,695
MONTH UNITS BILL PAYMENT
SEP-26 1.5 2,100 2,100
AUG-26 1.2 1,800 1,800
JUL-26 1.1 1,700 1,700
`
	r := ParseText(text, []Provider{{ShortName: "SNGPL"}}, "ocr")
	Validate(&r, Context{})
	wantFields(t, r, map[string]string{
		FReference: "12345678901", FBillingMonth: "2026-10", FIssueDate: "2026-10-06", FDueDate: "2026-10-19",
		FPreviousReading: "4512", FCurrentReading: "4530", FUnits: "18",
		FArrears: "0", FCurrentCharges: "245000", FPayable: "245000", FPayableAfterDue: "269500",
	})
	if len(r.History) != 3 || r.History[2].Month != "2026-09" || *r.History[2].Units != 1.5 || *r.History[2].BilledMinor != 210000 {
		t.Fatalf("history = %+v", r.History)
	}
}

func TestTablesTelecomOneRow(t *testing.T) {
	text := `Telephone & Broadband Bill
Account ID Bill Date Due Date Total Amount Due Late Payment Surcharge
A1234567 01/10/2026 15/10/2026 Rs. 4,999 Rs. 250
Name: SAMPLE CUSTOMER
`
	r := ParseText(text, nil, "ocr")
	Validate(&r, Context{})
	wantFields(t, r, map[string]string{
		FReference: "A1234567", FIssueDate: "2026-10-01", FDueDate: "2026-10-15", FPayable: "499900",
		FPayableAfterDue: "524900", FCustomerName: "SAMPLE CUSTOMER",
	})
}

func TestTablesWaterLabelValue(t *testing.T) {
	// "Label: value" pairs, one per line, still go through the generic parser.
	text := `WATER & SEWERAGE BOARD
Consumer Number: 0098765432
Bill Month: September 2026
Current Charges: 1,200
Arrears: 300
Total Amount Due: 1,500
Due Date: 20-Oct-2026
`
	r := ParseText(text, nil, "ocr")
	Validate(&r, Context{})
	wantFields(t, r, map[string]string{
		FReference: "0098765432", FBillingMonth: "2026-09", FCurrentCharges: "120000", FArrears: "30000",
		FPayable: "150000", FDueDate: "2026-10-20",
	})
}

func TestHeaderNeedsOnlyLabels(t *testing.T) {
	for _, line := range []string{
		"This is Gas Bill. Customer name and address mentioned herein",
		"MMBTU 2.920 GCV 950.5 Pressure Factor 1.0000",
		"BILL & PAYMENT HISTORY",
		"Due Date",
	} {
		if cols, ok := headerColumns(line); ok {
			t.Errorf("%q read as a header: %+v", line, cols)
		}
	}
}
