package fil

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type fil struct {
	locale                 string
	pluralsCardinal        []locales.PluralRule
	pluralsOrdinal         []locales.PluralRule
	pluralsRange           []locales.PluralRule
	decimal                string
	group                  string
	minus                  string
	percent                string
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

// New returns a new instance of translator for the 'fil' locale
func New() locales.Translator {
	return &fil{
		locale:                 "fil",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         []locales.PluralRule{2, 6},
		pluralsRange:           []locales.PluralRule{2, 6},
		decimal:                ".",
		group:                  ",",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "A$", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "R$", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CA$", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CN¥", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "€", "FIM", "FJD", "FKP", "FRF", "£", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HK$", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "₪", "₹", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "¥", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "₩", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MX$", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZ$", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "₱", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "฿", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "NT$", "TZS", "UAH", "UAK", "UGS", "UGX", "$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "₫", "VNN", "VUV", "WST", "FCFA", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "EC$", "XDR", "XEU", "XFO", "XFU", "CFA", "XPD", "CFPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: ")",
		monthsAbbreviated:      []string{"", "Ene", "Peb", "Mar", "Abr", "May", "Hun", "Hul", "Ago", "Set", "Okt", "Nob", "Dis"},
		monthsNarrow:           []string{"", "Ene", "Peb", "Mar", "Abr", "May", "Hun", "Hul", "Ago", "Set", "Okt", "Nob", "Dis"},
		monthsWide:             []string{"", "Enero", "Pebrero", "Marso", "Abril", "Mayo", "Hunyo", "Hulyo", "Agosto", "Setyembre", "Oktubre", "Nobyembre", "Disyembre"},
		daysAbbreviated:        []string{"Lin", "Lun", "Mar", "Miy", "Huw", "Biy", "Sab"},
		daysNarrow:             []string{"Lin", "Lun", "Mar", "Miy", "Huw", "Biy", "Sab"},
		daysShort:              []string{"Li", "Lu", "Ma", "Mi", "Hu", "Bi", "Sa"},
		daysWide:               []string{"Linggo", "Lunes", "Martes", "Miyerkules", "Huwebes", "Biyernes", "Sabado"},
		periodsAbbreviated:     []string{"AM", "PM"},
		periodsNarrow:          []string{"am", "pm"},
		periodsWide:            []string{"AM", "PM"},
		erasAbbreviated:        []string{"BC", "AD"},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"BC", "AD"},
		timezones:              map[string]string{"AST": "Standard na Oras sa Atlantiko", "EAT": "Oras sa Silangang Africa", "HKT": "Standard na Oras sa Hong Kong", "JDT": "Daylight Time sa Japan", "WAT": "Standard na Oras sa Kanlurang Africa", "AKDT": "Daylight Time sa Alaska", "CHAST": "Standard na Oras sa Chatham", "ACWST": "Standard Time ng Gitnang Kanluran ng Australya", "SAST": "Oras sa Timog Africa", "MDT": "Daylight Time sa Bundok", "COT": "Standard na Oras sa Colombia", "MYT": "Oras ng Malaysia", "WARST": "Oras sa Tag-init ng Kanlurang Argentina", "ChST": "Standard na Oras sa Chamorro", "AWDT": "Daylight Time sa Kanlurang Australya", "HNOG": "Standard na Oras sa Kanlurang Greenland", "EDT": "Eastern Daylight Time", "CAT": "Oras sa Gitnang Africa", "HENOMX": "Daylight Time sa Hilagang-kanlurang Mexico", "JST": "Standard na Oras sa Japan", "HEOG": "Oras sa Tag-init ng Kanlurang Greenland", "ACDT": "Daylight Time ng Gitnang Australya", "SGT": "Standard na Oras sa Singapore", "CST": "Sentral na Karaniwang Oras", "ACWDT": "Daylight Time ng Gitnang Kanluran ng Australya", "VET": "Oras sa Venezuela", "ADT": "Daylight Time sa Atlantiko", "HAT": "Daylight Time sa Newfoundland", "WEZ": "Standard na Oras sa Kanlurang Europe", "WESZ": "Oras sa Tag-init ng Kanlurang Europe", "HEPM": "Daylight Time sa Saint Pierre and Miquelon", "NZDT": "Daylight Time sa New Zealand", "LHST": "Standard na Oras sa Lord Howe", "∅∅∅": "Oras sa Tag-init ng Azores", "CLT": "Standard na Oras sa Chile", "EST": "Eastern na Standard na Oras", "CHADT": "Daylight Time sa Chatham", "WIT": "Oras ng Silangang Indonesiya", "LHDT": "Daylight Time sa Lorde Howe", "CDT": "Sentral na Daylight Time", "NZST": "Standard na Oras sa New Zealand", "AEST": "Standard Time ng Silangang Australya", "HNEG": "Standard na Oras sa Silangang Greenland", "GFT": "Oras sa French Guiana", "HNT": "Standard na Oras sa Newfoundland", "HKST": "Oras sa Tag-init ng Hong Kong", "HEPMX": "Daylight Time sa Pasipiko ng Mexico", "HECU": "Daylight Time sa Cuba", "HAST": "Standard na Oras sa Hawaii-Aleutian", "HNNOMX": "Standard na Oras sa Hilagang-kanlurang Mexico", "HADT": "Oras sa Tag-init ng Hawaii-Aleutian", "HEEG": "Oras sa Tag-init ng Silangang Greenland", "CLST": "Oras sa Tag-init ng Chile", "AKST": "Standard na Oras sa Alaska", "GMT": "Greenwich Mean Time", "SRT": "Oras sa Suriname", "UYST": "Oras sa Tag-init ng Uruguay", "AEDT": "Daylight Time ng Silangang Australya", "OESZ": "Oras sa Tag-init ng Silangang Europe", "HNPMX": "Standard na Oras sa Pasipiko ng Mexico", "HNPM": "Standard na Oras sa Saint Pierre and Miquelon", "BOT": "Oras sa Bolivia", "WITA": "Oras ng Gitnang Indonesiya", "ARST": "Oras sa Tag-init ng Argentina", "WAST": "Oras sa Tag-init ng Kanlurang Africa", "HNCU": "Standard na Oras sa Cuba", "TMT": "Standard na Oras sa Turkmenistan", "TMST": "Oras sa Tag-init ng Turkmenistan", "ART": "Standard na Oras sa Argentina", "MST": "Karaniwang Oras sa Bundok", "GYT": "Oras sa Guyana", "PST": "Standard na Oras sa Pasipiko", "OEZ": "Standard na Oras sa Silangang Europe", "IST": "Standard na Oras sa Bhutan", "WIB": "Oras ng Kanlurang Indonesiya", "UYT": "Standard na Oras sa Uruguay", "MESZ": "Oras sa Tag-init ng Gitnang Europe", "WART": "Standard na Oras sa Kanlurang Argentina", "ACST": "Standard Time ng Gitnang Australya", "ECT": "Oras sa Ecuador", "PDT": "Daylight Time sa Pasipiko", "BT": "Oras sa Bhutan", "AWST": "Standard Time ng Kanlurang Australya", "MEZ": "Standard na Oras sa Gitnang Europe", "COST": "Oras sa Tag-init ng Colombia"},
	}
}

// Locale returns the current translators string locale
func (fil *fil) Locale() string {
	return fil.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'fil'
func (fil *fil) PluralsCardinal() []locales.PluralRule {
	return fil.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'fil'
func (fil *fil) PluralsOrdinal() []locales.PluralRule {
	return fil.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'fil'
func (fil *fil) PluralsRange() []locales.PluralRule {
	return fil.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'fil'
func (fil *fil) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)
	f := locales.F(n, v)
	iMod10 := i % 10
	fMod10 := f % 10

	if (v == 0 && (i == 1 || i == 2 || i == 3)) || (v == 0 && (iMod10 != 4 && iMod10 != 6 && iMod10 != 9)) || (v != 0 && (fMod10 != 4 && fMod10 != 6 && fMod10 != 9)) {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'fil'
func (fil *fil) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'fil'
func (fil *fil) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {

	start := fil.CardinalPluralRule(num1, v1)
	end := fil.CardinalPluralRule(num2, v2)

	if start == locales.PluralRuleOne && end == locales.PluralRuleOne {
		return locales.PluralRuleOne
	} else if start == locales.PluralRuleOne && end == locales.PluralRuleOther {
		return locales.PluralRuleOther
	} else if start == locales.PluralRuleOther && end == locales.PluralRuleOne {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther

}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (fil *fil) MonthAbbreviated(month time.Month) string {
	return fil.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (fil *fil) MonthsAbbreviated() []string {
	return fil.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (fil *fil) MonthNarrow(month time.Month) string {
	return fil.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (fil *fil) MonthsNarrow() []string {
	return fil.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (fil *fil) MonthWide(month time.Month) string {
	return fil.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (fil *fil) MonthsWide() []string {
	return fil.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (fil *fil) WeekdayAbbreviated(weekday time.Weekday) string {
	return fil.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (fil *fil) WeekdaysAbbreviated() []string {
	return fil.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (fil *fil) WeekdayNarrow(weekday time.Weekday) string {
	return fil.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (fil *fil) WeekdaysNarrow() []string {
	return fil.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (fil *fil) WeekdayShort(weekday time.Weekday) string {
	return fil.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (fil *fil) WeekdaysShort() []string {
	return fil.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (fil *fil) WeekdayWide(weekday time.Weekday) string {
	return fil.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (fil *fil) WeekdaysWide() []string {
	return fil.daysWide
}

// Decimal returns the decimal point of number
func (fil *fil) Decimal() string {
	return fil.decimal
}

// Group returns the group of number
func (fil *fil) Group() string {
	return fil.group
}

// Group returns the minus sign of number
func (fil *fil) Minus() string {
	return fil.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'fil' and handles both Whole and Real numbers based on 'v'
func (fil *fil) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, fil.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, fil.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, fil.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'fil' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (fil *fil) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 3
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, fil.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, fil.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, fil.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'fil'
func (fil *fil) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := fil.currencies[currency]
	l := len(s) + len(symbol) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, fil.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, fil.group[0])
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
		b = append(b, fil.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, fil.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'fil'
// in accounting notation.
func (fil *fil) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := fil.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, fil.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, fil.group[0])
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

		b = append(b, fil.currencyNegativePrefix[0])

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
			b = append(b, fil.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, fil.currencyNegativeSuffix...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'fil'
func (fil *fil) FmtDateShort(t time.Time) string {

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

// FmtDateMedium returns the medium date representation of 't' for 'fil'
func (fil *fil) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, fil.monthsAbbreviated[t.Month()]...)
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

// FmtDateLong returns the long date representation of 't' for 'fil'
func (fil *fil) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, fil.monthsWide[t.Month()]...)
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

// FmtDateFull returns the full date representation of 't' for 'fil'
func (fil *fil) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, fil.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = append(b, fil.monthsWide[t.Month()]...)
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

// FmtTimeShort returns the short time representation of 't' for 'fil'
func (fil *fil) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, fil.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, fil.periodsAbbreviated[0]...)
	} else {
		b = append(b, fil.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'fil'
func (fil *fil) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, fil.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, fil.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, fil.periodsAbbreviated[0]...)
	} else {
		b = append(b, fil.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'fil'
func (fil *fil) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, fil.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, fil.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, fil.periodsAbbreviated[0]...)
	} else {
		b = append(b, fil.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'fil'
func (fil *fil) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, fil.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, fil.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, fil.periodsAbbreviated[0]...)
	} else {
		b = append(b, fil.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := fil.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
