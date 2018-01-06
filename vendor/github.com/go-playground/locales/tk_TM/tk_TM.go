package tk_TM

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type tk_TM struct {
	locale                 string
	pluralsCardinal        []locales.PluralRule
	pluralsOrdinal         []locales.PluralRule
	pluralsRange           []locales.PluralRule
	decimal                string
	group                  string
	minus                  string
	percent                string
	percentSuffix          string
	perMille               string
	timeSeparator          string
	inifinity              string
	currencies             []string // idx = enum of currency code
	currencyPositiveSuffix string
	currencyNegativeSuffix string
	monthsAbbreviated      []string
	monthsNarrow           []string
	monthsWide             []string
	daysAbbreviated        []string
	daysNarrow             []string
	daysShort              []string
	daysWide               []string
	periodsAbbreviated     []string
	periodsNarrow          []string
	periodsShort           []string
	periodsWide            []string
	erasAbbreviated        []string
	erasNarrow             []string
	erasWide               []string
	timezones              map[string]string
}

// New returns a new instance of translator for the 'tk_TM' locale
func New() locales.Translator {
	return &tk_TM{
		locale:                 "tk_TM",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		decimal:                ",",
		group:                  " ",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:          " ",
		currencyPositiveSuffix: " ",
		currencyNegativeSuffix: " ",
		monthsAbbreviated:      []string{"", "ýan", "few", "mart", "apr", "maý", "iýun", "iýul", "awg", "sen", "okt", "noý", "dek"},
		monthsNarrow:           []string{"", "Ý", "F", "M", "A", "M", "I", "I", "A", "S", "O", "N", "D"},
		monthsWide:             []string{"", "ýanwar", "fewral", "mart", "aprel", "maý", "iýun", "iýul", "awgust", "sentýabr", "oktýabr", "noýabr", "dekabr"},
		daysAbbreviated:        []string{"ýb", "db", "sb", "çb", "pb", "an", "şb"},
		daysNarrow:             []string{"Ý", "D", "S", "Ç", "P", "A", "Ş"},
		daysWide:               []string{"ýekşenbe", "duşenbe", "sişenbe", "çarşenbe", "penşenbe", "anna", "şenbe"},
		timezones:              map[string]string{"PST": "Ýuwaş umman, standart wagt", "AST": "Atlantika, standart wagt", "CLT": "Çili, standart wagt", "∅∅∅": "Peru, tomusky wagt", "HNPMX": "Meksikan Ýuwaş umman, standart wagt", "ACWDT": "Merkezi Awstraliýa, günbatar tarap, tomusky wagt", "HNEG": "Gündogar Grenlandiýa, standart wagt", "JDT": "Ýaponiýa, tomusky wagt", "HKT": "Gonkong, standart wagt", "CAT": "Merkezi Afrika", "WIB": "Günbatar Indoneziýa", "CHADT": "Çatem, tomusky wagt", "HEPM": "Sen Pýer we Mikelon, tomusky wagt", "PDT": "Ýuwaş umman, tomusky wagt", "SGT": "Singapur, standart wagt", "CDT": "Merkezi Amerika, tomusky wagt", "ACST": "Merkezi Awstraliýa, standart wagt", "WEZ": "Günbatar Ýewropa, standart wagt", "WESZ": "Günbatar Ýewropa, tomusky wagt", "ChST": "Çamorro", "HEPMX": "Meksikan Ýuwaş umman, tomusky wagt", "UYST": "Urugwaý, tomusky wagt", "WAST": "Günbatar Afrika, tomusky wagt", "EAT": "Gündogar Afrika", "GFT": "Fransuz Gwiana", "GYT": "Gaýana", "HECU": "Kuba, tomusky wagt", "IST": "Hindistan", "ART": "Argentina, standart wagt", "HNOG": "Günbatar Grenlandiýa, standart wagt", "HEEG": "Gündogar Grenlandiýa, tomusky wagt", "ACDT": "Merkezi Awstraliýa, tomusky wagt", "AKST": "Alýaska, standart wagt", "MDT": "MDT", "SRT": "Surinam", "UYT": "Urugwaý, standart wagt", "WART": "Günbatar Argentina, standart wagt", "MESZ": "Merkezi Ýewropa, tomusky wagt", "LHST": "Lord-Hau, standart wagt", "WITA": "Merkezi Indoneziýa", "ADT": "Atlantika, tomusky wagt", "HNCU": "Kuba, standart wagt", "HAST": "Gawaý-Aleut, standart wagt", "NZST": "Täze Zelandiýa, standart wagt", "TMT": "Türkmenistan, standart wagt", "AKDT": "Alýaska, tomusky wagt", "WARST": "Günbatar Argentina, tomusky wagt", "COST": "Kolumbiýa, tomusky wagt", "EST": "Günorta Amerika, standart wagt", "CST": "Merkezi Amerika, standart wagt", "ACWST": "Merkezi Awstraliýa, günbatar tarap, standart wagt", "AEST": "Gündogar Awstraliýa, standart wagt", "COT": "Kolumbiýa, standart wagt", "CLST": "Çili, tomusky wagt", "EDT": "Günorta Amerika, tomusky wagt", "GMT": "Grinwiç boýunça orta wagt", "CHAST": "Çatem, standart wagt", "HNPM": "Sen Pýer we Mikelon, standart wagt", "HADT": "Gawaý-Aleut, tomusky wagt", "AWST": "Günbatar Awstraliýa, standart wagt", "JST": "Ýaponiýa, standart wagt", "HAT": "Nýufaundlend, tomusky wagt", "HNT": "Nýufaundlend, standart wagt", "BT": "Butan", "MYT": "Malaýziýa", "WIT": "Gündogar Indoneziýa", "SAST": "Günorta Afrika, standart wagt", "HEOG": "Günbatar Grenlandiýa, tomusky wagt", "AWDT": "Günbatar Awstraliýa, tomusky wagt", "NZDT": "Täze Zelandiýa, tomusky wagt", "VET": "Wenesuela", "ARST": "Argentina, tomusky wagt", "AEDT": "Gündogar Awstraliýa, tomusky wagt", "MST": "MST", "TMST": "Türkmenistan, tomusky wagt", "LHDT": "Lord-Hau, tomusky wagt", "HENOMX": "D.g.-G.b. Meksika, tomusky wagt", "OESZ": "Gündogar Ýewropa, tomusky wagt", "WAT": "Günbatar Afrika, standart wagt", "HKST": "Gonkong, tomusky wagt", "ECT": "Ekwador", "BOT": "Boliwiýa", "MEZ": "Merkezi Ýewropa, standart wagt", "HNNOMX": "D.g.-G.b. Meksika, standart wagt", "OEZ": "Gündogar Ýewropa, standart wagt"},
	}
}

// Locale returns the current translators string locale
func (tk *tk_TM) Locale() string {
	return tk.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'tk_TM'
func (tk *tk_TM) PluralsCardinal() []locales.PluralRule {
	return tk.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'tk_TM'
func (tk *tk_TM) PluralsOrdinal() []locales.PluralRule {
	return tk.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'tk_TM'
func (tk *tk_TM) PluralsRange() []locales.PluralRule {
	return tk.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'tk_TM'
func (tk *tk_TM) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'tk_TM'
func (tk *tk_TM) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'tk_TM'
func (tk *tk_TM) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (tk *tk_TM) MonthAbbreviated(month time.Month) string {
	return tk.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (tk *tk_TM) MonthsAbbreviated() []string {
	return tk.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (tk *tk_TM) MonthNarrow(month time.Month) string {
	return tk.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (tk *tk_TM) MonthsNarrow() []string {
	return tk.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (tk *tk_TM) MonthWide(month time.Month) string {
	return tk.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (tk *tk_TM) MonthsWide() []string {
	return tk.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (tk *tk_TM) WeekdayAbbreviated(weekday time.Weekday) string {
	return tk.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (tk *tk_TM) WeekdaysAbbreviated() []string {
	return tk.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (tk *tk_TM) WeekdayNarrow(weekday time.Weekday) string {
	return tk.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (tk *tk_TM) WeekdaysNarrow() []string {
	return tk.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (tk *tk_TM) WeekdayShort(weekday time.Weekday) string {
	return tk.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (tk *tk_TM) WeekdaysShort() []string {
	return tk.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (tk *tk_TM) WeekdayWide(weekday time.Weekday) string {
	return tk.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (tk *tk_TM) WeekdaysWide() []string {
	return tk.daysWide
}

// Decimal returns the decimal point of number
func (tk *tk_TM) Decimal() string {
	return tk.decimal
}

// Group returns the group of number
func (tk *tk_TM) Group() string {
	return tk.group
}

// Group returns the minus sign of number
func (tk *tk_TM) Minus() string {
	return tk.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'tk_TM' and handles both Whole and Real numbers based on 'v'
func (tk *tk_TM) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, tk.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(tk.group) - 1; j >= 0; j-- {
					b = append(b, tk.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, tk.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'tk_TM' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (tk *tk_TM) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 5
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, tk.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, tk.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, tk.percentSuffix...)

	b = append(b, tk.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'tk_TM'
func (tk *tk_TM) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := tk.currencies[currency]
	l := len(s) + len(symbol) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, tk.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(tk.group) - 1; j >= 0; j-- {
					b = append(b, tk.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, tk.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, tk.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, tk.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'tk_TM'
// in accounting notation.
func (tk *tk_TM) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := tk.currencies[currency]
	l := len(s) + len(symbol) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, tk.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(tk.group) - 1; j >= 0; j-- {
					b = append(b, tk.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, tk.minus[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, tk.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, tk.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, tk.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'tk_TM'
func (tk *tk_TM) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2e}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'tk_TM'
func (tk *tk_TM) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, tk.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'tk_TM'
func (tk *tk_TM) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, tk.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'tk_TM'
func (tk *tk_TM) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, tk.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20}...)
	b = append(b, tk.daysWide[t.Weekday()]...)

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'tk_TM'
func (tk *tk_TM) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, tk.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'tk_TM'
func (tk *tk_TM) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, tk.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, tk.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'tk_TM'
func (tk *tk_TM) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, tk.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, tk.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'tk_TM'
func (tk *tk_TM) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, tk.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, tk.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := tk.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
