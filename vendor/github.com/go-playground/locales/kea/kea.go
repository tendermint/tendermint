package kea

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type kea struct {
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
	currencyPositiveSuffix string
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

// New returns a new instance of translator for the 'kea' locale
func New() locales.Translator {
	return &kea{
		locale:                 "kea",
		pluralsCardinal:        []locales.PluralRule{6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		decimal:                ",",
		group:                  " ",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AU$", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "৳", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "$", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "R$", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CA$", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CN¥", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "\u200b", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "€", "FIM", "$", "FKP", "FRF", "£", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HK$", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "₪", "₹", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JP¥", "KES", "KGS", "៛", "KMF", "KPW", "KRH", "KRO", "₩", "KWD", "KYD", "₸", "₭", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "₮", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MX$", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZ$", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "₱", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "$", "SCR", "SDD", "SDG", "SDP", "SEK", "$", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "฿", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "₺", "TTD", "NT$", "TZS", "UAH", "UAK", "UGS", "UGX", "US$", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "₫", "VNN", "VUV", "WST", "FCFA", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "EC$", "XDR", "XEU", "XFO", "XFU", "CFA", "XPD", "CFPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositiveSuffix: " ",
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: " )",
		monthsAbbreviated:      []string{"", "Jan", "Feb", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Otu", "Nuv", "Diz"},
		monthsNarrow:           []string{"", "J", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"},
		monthsWide:             []string{"", "Janeru", "Febreru", "Marsu", "Abril", "Maiu", "Junhu", "Julhu", "Agostu", "Setenbru", "Otubru", "Nuvenbru", "Dizenbru"},
		daysAbbreviated:        []string{"dum", "sig", "ter", "kua", "kin", "ses", "sab"},
		daysNarrow:             []string{"D", "S", "T", "K", "K", "S", "S"},
		daysShort:              []string{"du", "si", "te", "ku", "ki", "se", "sa"},
		daysWide:               []string{"dumingu", "sigunda-fera", "tersa-fera", "kuarta-fera", "kinta-fera", "sesta-fera", "sabadu"},
		periodsAbbreviated:     []string{"am", "pm"},
		periodsNarrow:          []string{"a", "p"},
		periodsWide:            []string{"am", "pm"},
		erasAbbreviated:        []string{"AK", "DK"},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"Antis di Kristu", "Dispos di Kristu"},
		timezones:              map[string]string{"JDT": "JDT", "ART": "ART", "HEPMX": "HEPMX", "UYST": "UYST", "NZDT": "NZDT", "WARST": "WARST", "NZST": "NZST", "GYT": "GYT", "ACDT": "Ora di Verãu di Australia Sentral", "SGT": "SGT", "GMT": "GMT", "PDT": "Ora di Pasifiku di Verãu", "HADT": "HADT", "COT": "COT", "CAT": "Ora di Afrika Sentral", "CHADT": "CHADT", "SRT": "SRT", "WART": "WART", "ACST": "Ora Padrãu di Australia Sentral", "AKDT": "AKDT", "∅∅∅": "∅∅∅", "HNOG": "HNOG", "COST": "COST", "HNT": "HNT", "AEST": "Ora Padrãu di Australia Oriental", "SAST": "Ora di Sul di Afrika", "WIB": "WIB", "CHAST": "CHAST", "HNPM": "HNPM", "AWST": "Ora Padrãu di Australia Osidental", "JST": "JST", "AST": "Ora Padrãu di Atlantiku", "ADT": "Ora di Verãu di Atlantiku", "HAT": "HAT", "BOT": "BOT", "AWDT": "Ora di Verãu di Australia Osidental", "MYT": "MYT", "UYT": "UYT", "WAST": "Ora di Verão di Afrika Osidental", "HKST": "HKST", "CLT": "CLT", "GFT": "GFT", "HNCU": "HNCU", "MST": "MST", "AEDT": "Ora di Verãu di Australia Oriental", "HEOG": "HEOG", "ECT": "ECT", "HEEG": "HEEG", "ACWST": "Ora Padrãu di Australia Sentru-Osidental", "MESZ": "Ora di Verãu di Europa Sentral", "OESZ": "Ora di Verãu di Europa Oriental", "IST": "IST", "HNPMX": "HNPMX", "BT": "BT", "OEZ": "Ora Padrãu di Europa Oriental", "EST": "Ora Oriental Padrãu", "CLST": "CLST", "CST": "Ora Sentral Padrãu", "ACWDT": "Ora di Verãu di Australia Sentru-Osidental", "ARST": "ARST", "WAT": "Ora Padrãu di Afrika Osidental", "EAT": "Ora di Afrika Oriental", "EDT": "Ora Oriental di Verãu", "HECU": "HECU", "TMST": "TMST", "VET": "VET", "HNNOMX": "HNNOMX", "WIT": "WIT", "TMT": "TMT", "LHDT": "LHDT", "HKT": "HKT", "WITA": "WITA", "HNEG": "HNEG", "WESZ": "Ora di Verãu di Europa Osidental", "ChST": "ChST", "HEPM": "HEPM", "MDT": "MDT", "WEZ": "Ora Padrãu di Europa Osidental", "LHST": "LHST", "HAST": "HAST", "AKST": "AKST", "PST": "Ora di Pasifiku Padrãu", "CDT": "Ora Sentral di Verãu", "MEZ": "Ora Padrãu di Europa Sentral", "HENOMX": "HENOMX"},
	}
}

// Locale returns the current translators string locale
func (kea *kea) Locale() string {
	return kea.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'kea'
func (kea *kea) PluralsCardinal() []locales.PluralRule {
	return kea.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'kea'
func (kea *kea) PluralsOrdinal() []locales.PluralRule {
	return kea.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'kea'
func (kea *kea) PluralsRange() []locales.PluralRule {
	return kea.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'kea'
func (kea *kea) CardinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'kea'
func (kea *kea) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'kea'
func (kea *kea) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (kea *kea) MonthAbbreviated(month time.Month) string {
	return kea.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (kea *kea) MonthsAbbreviated() []string {
	return kea.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (kea *kea) MonthNarrow(month time.Month) string {
	return kea.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (kea *kea) MonthsNarrow() []string {
	return kea.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (kea *kea) MonthWide(month time.Month) string {
	return kea.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (kea *kea) MonthsWide() []string {
	return kea.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (kea *kea) WeekdayAbbreviated(weekday time.Weekday) string {
	return kea.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (kea *kea) WeekdaysAbbreviated() []string {
	return kea.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (kea *kea) WeekdayNarrow(weekday time.Weekday) string {
	return kea.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (kea *kea) WeekdaysNarrow() []string {
	return kea.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (kea *kea) WeekdayShort(weekday time.Weekday) string {
	return kea.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (kea *kea) WeekdaysShort() []string {
	return kea.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (kea *kea) WeekdayWide(weekday time.Weekday) string {
	return kea.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (kea *kea) WeekdaysWide() []string {
	return kea.daysWide
}

// Decimal returns the decimal point of number
func (kea *kea) Decimal() string {
	return kea.decimal
}

// Group returns the group of number
func (kea *kea) Group() string {
	return kea.group
}

// Group returns the minus sign of number
func (kea *kea) Minus() string {
	return kea.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'kea' and handles both Whole and Real numbers based on 'v'
func (kea *kea) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kea.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(kea.group) - 1; j >= 0; j-- {
					b = append(b, kea.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, kea.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'kea' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (kea *kea) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 3
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kea.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, kea.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, kea.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'kea'
func (kea *kea) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := kea.currencies[currency]
	l := len(s) + len(symbol) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kea.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(kea.group) - 1; j >= 0; j-- {
					b = append(b, kea.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, kea.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, kea.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, kea.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'kea'
// in accounting notation.
func (kea *kea) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := kea.currencies[currency]
	l := len(s) + len(symbol) + 6 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, kea.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(kea.group) - 1; j >= 0; j-- {
					b = append(b, kea.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, kea.currencyNegativePrefix[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, kea.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, kea.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, kea.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'kea'
func (kea *kea) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'kea'
func (kea *kea) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, kea.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'kea'
func (kea *kea) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20, 0x64, 0x69}...)
	b = append(b, []byte{0x20}...)
	b = append(b, kea.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20, 0x64, 0x69}...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'kea'
func (kea *kea) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, kea.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20, 0x64, 0x69}...)
	b = append(b, []byte{0x20}...)
	b = append(b, kea.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20, 0x64, 0x69}...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'kea'
func (kea *kea) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, kea.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'kea'
func (kea *kea) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, kea.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kea.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'kea'
func (kea *kea) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, kea.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kea.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'kea'
func (kea *kea) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, kea.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, kea.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := kea.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
