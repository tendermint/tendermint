package en_DE

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type en_DE struct {
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
	currencyNegativePrefix string
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

// New returns a new instance of translator for the 'en_DE' locale
func New() locales.Translator {
	return &en_DE{
		locale:                 "en_DE",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         []locales.PluralRule{2, 3, 4, 6},
		pluralsRange:           []locales.PluralRule{6},
		decimal:                ".",
		group:                  ",",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:          " ",
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: ")",
		monthsAbbreviated:      []string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		monthsNarrow:           []string{"", "J", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"},
		monthsWide:             []string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
		daysAbbreviated:        []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		daysNarrow:             []string{"S", "M", "T", "W", "T", "F", "S"},
		daysShort:              []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"},
		daysWide:               []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
		periodsAbbreviated:     []string{"am", "pm"},
		periodsNarrow:          []string{"a", "p"},
		periodsWide:            []string{"am", "pm"},
		erasAbbreviated:        []string{"BC", "AD"},
		erasNarrow:             []string{"B", "A"},
		erasWide:               []string{"Before Christ", "Anno Domini"},
		timezones:              map[string]string{"CLST": "Chile Summer Time", "HNT": "Newfoundland Standard Time", "HAT": "Newfoundland Daylight Time", "EDT": "Eastern Daylight Time", "MDT": "Macau Summer Time", "JST": "Japan Standard Time", "ARST": "Argentina Summer Time", "ADT": "Atlantic Daylight Time", "HEPM": "St. Pierre & Miquelon Daylight Time", "BOT": "Bolivia Time", "NZDT": "New Zealand Daylight Time", "LHDT": "Lord Howe Daylight Time", "AEDT": "Australian Eastern Daylight Time", "HNEG": "East Greenland Standard Time", "HNPM": "St. Pierre & Miquelon Standard Time", "AWDT": "Australian Western Daylight Time", "WART": "Western Argentina Standard Time", "HKST": "Hong Kong Summer Time", "AKST": "Alaska Standard Time", "VET": "Venezuela Time", "HENOMX": "Northwest Mexico Daylight Time", "HEOG": "West Greenland Summer Time", "WAST": "West Africa Summer Time", "MEZ": "Central European Standard Time", "WITA": "Central Indonesia Time", "OEZ": "Eastern European Standard Time", "IST": "India Standard Time", "COT": "Colombia Standard Time", "CHADT": "Chatham Daylight Time", "CLT": "Chile Standard Time", "ChST": "Chamorro Standard Time", "HNPMX": "Mexican Pacific Standard Time", "AWST": "Australian Western Standard Time", "SRT": "Suriname Time", "HNNOMX": "Northwest Mexico Standard Time", "ART": "Argentina Standard Time", "HEEG": "East Greenland Summer Time", "SGT": "Singapore Standard Time", "WIB": "Western Indonesia Time", "BT": "Bhutan Time", "EST": "Eastern Standard Time", "ACST": "Australian Central Standard Time", "PDT": "Pacific Daylight Time", "UYT": "Uruguay Standard Time", "TMT": "Turkmenistan Standard Time", "WARST": "Western Argentina Summer Time", "SAST": "South Africa Standard Time", "AKDT": "Alaska Daylight Time", "GMT": "Greenwich Mean Time", "HNCU": "Cuba Standard Time", "HECU": "Cuba Daylight Time", "MST": "Macau Standard Time", "LHST": "Lord Howe Standard Time", "WAT": "West Africa Standard Time", "CAT": "Central Africa Time", "CST": "Central Standard Time", "HKT": "Hong Kong Standard Time", "GYT": "Guyana Time", "EAT": "East Africa Time", "COST": "Colombia Summer Time", "GFT": "French Guiana Time", "ACDT": "Australian Central Daylight Time", "ECT": "Ecuador Time", "WEZ": "Western European Standard Time", "HNOG": "West Greenland Standard Time", "AST": "Atlantic Standard Time", "WIT": "Eastern Indonesia Time", "OESZ": "Eastern European Summer Time", "HEPMX": "Mexican Pacific Daylight Time", "PST": "Pacific Standard Time", "NZST": "New Zealand Standard Time", "TMST": "Turkmenistan Summer Time", "MESZ": "Central European Summer Time", "WESZ": "Western European Summer Time", "ACWST": "Australian Central Western Standard Time", "UYST": "Uruguay Summer Time", "HADT": "Hawaii-Aleutian Daylight Time", "CDT": "Central Daylight Time", "MYT": "Malaysia Time", "CHAST": "Chatham Standard Time", "ACWDT": "Australian Central Western Daylight Time", "HAST": "Hawaii-Aleutian Standard Time", "JDT": "Japan Daylight Time", "AEST": "Australian Eastern Standard Time", "∅∅∅": "Amazon Summer Time"},
	}
}

// Locale returns the current translators string locale
func (en *en_DE) Locale() string {
	return en.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'en_DE'
func (en *en_DE) PluralsCardinal() []locales.PluralRule {
	return en.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'en_DE'
func (en *en_DE) PluralsOrdinal() []locales.PluralRule {
	return en.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'en_DE'
func (en *en_DE) PluralsRange() []locales.PluralRule {
	return en.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'en_DE'
func (en *en_DE) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if i == 1 && v == 0 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'en_DE'
func (en *en_DE) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	nMod10 := math.Mod(n, 10)
	nMod100 := math.Mod(n, 100)

	if nMod10 == 1 && nMod100 != 11 {
		return locales.PluralRuleOne
	} else if nMod10 == 2 && nMod100 != 12 {
		return locales.PluralRuleTwo
	} else if nMod10 == 3 && nMod100 != 13 {
		return locales.PluralRuleFew
	}

	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'en_DE'
func (en *en_DE) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (en *en_DE) MonthAbbreviated(month time.Month) string {
	return en.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (en *en_DE) MonthsAbbreviated() []string {
	return en.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (en *en_DE) MonthNarrow(month time.Month) string {
	return en.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (en *en_DE) MonthsNarrow() []string {
	return en.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (en *en_DE) MonthWide(month time.Month) string {
	return en.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (en *en_DE) MonthsWide() []string {
	return en.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (en *en_DE) WeekdayAbbreviated(weekday time.Weekday) string {
	return en.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (en *en_DE) WeekdaysAbbreviated() []string {
	return en.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (en *en_DE) WeekdayNarrow(weekday time.Weekday) string {
	return en.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (en *en_DE) WeekdaysNarrow() []string {
	return en.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (en *en_DE) WeekdayShort(weekday time.Weekday) string {
	return en.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (en *en_DE) WeekdaysShort() []string {
	return en.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (en *en_DE) WeekdayWide(weekday time.Weekday) string {
	return en.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (en *en_DE) WeekdaysWide() []string {
	return en.daysWide
}

// Decimal returns the decimal point of number
func (en *en_DE) Decimal() string {
	return en.decimal
}

// Group returns the group of number
func (en *en_DE) Group() string {
	return en.group
}

// Group returns the minus sign of number
func (en *en_DE) Minus() string {
	return en.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'en_DE' and handles both Whole and Real numbers based on 'v'
func (en *en_DE) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, en.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, en.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, en.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'en_DE' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (en *en_DE) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 5
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, en.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, en.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, en.percentSuffix...)

	b = append(b, en.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'en_DE'
func (en *en_DE) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := en.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, en.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, en.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	for j := len(symbol) - 1; j >= 0; j-- {
		b = append(b, symbol[j])
	}

	if num < 0 {
		b = append(b, en.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, en.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'en_DE'
// in accounting notation.
func (en *en_DE) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := en.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, en.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, en.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

		b = append(b, en.currencyNegativePrefix[0])

	} else {

		for j := len(symbol) - 1; j >= 0; j-- {
			b = append(b, symbol[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, en.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, en.currencyNegativeSuffix...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'en_DE'
func (en *en_DE) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'en_DE'
func (en *en_DE) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, en.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'en_DE'
func (en *en_DE) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, en.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'en_DE'
func (en *en_DE) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, en.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = append(b, en.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'en_DE'
func (en *en_DE) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, en.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, en.periodsAbbreviated[0]...)
	} else {
		b = append(b, en.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'en_DE'
func (en *en_DE) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, en.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, en.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, en.periodsAbbreviated[0]...)
	} else {
		b = append(b, en.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'en_DE'
func (en *en_DE) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, en.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, en.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, en.periodsAbbreviated[0]...)
	} else {
		b = append(b, en.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'en_DE'
func (en *en_DE) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, en.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, en.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, en.periodsAbbreviated[0]...)
	} else {
		b = append(b, en.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := en.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
