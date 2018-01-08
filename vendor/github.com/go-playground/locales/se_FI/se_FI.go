package se_FI

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type se_FI struct {
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

// New returns a new instance of translator for the 'se_FI' locale
func New() locales.Translator {
	return &se_FI{
		locale:                 "se_FI",
		pluralsCardinal:        []locales.PluralRule{2, 3, 6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		decimal:                ",",
		group:                  " ",
		minus:                  "−",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:          " ",
		currencyPositiveSuffix: " ",
		currencyNegativeSuffix: " ",
		monthsAbbreviated:      []string{"", "ođđj", "guov", "njuk", "cuo", "mies", "geas", "suoi", "borg", "čakč", "golg", "skáb", "juov"},
		monthsNarrow:           []string{"", "O", "G", "N", "C", "M", "G", "S", "B", "Č", "G", "S", "J"},
		monthsWide:             []string{"", "ođđajagemánnu", "guovvamánnu", "njukčamánnu", "cuoŋománnu", "miessemánnu", "geassemánnu", "suoidnemánnu", "borgemánnu", "čakčamánnu", "golggotmánnu", "skábmamánnu", "juovlamánnu"},
		daysAbbreviated:        []string{"sotn", "vuos", "maŋ", "gask", "duor", "bear", "láv"},
		daysNarrow:             []string{"S", "M", "D", "G", "D", "B", "L"},
		daysShort:              []string{"sotn", "vuos", "maŋ", "gask", "duor", "bear", "láv"},
		daysWide:               []string{"sotnabeaivi", "vuossárgga", "maŋŋebárgga", "gaskavahku", "duorastaga", "bearjadaga", "lávvardaga"},
		periodsAbbreviated:     []string{"i.b.", "e.b."},
		periodsNarrow:          []string{"i.b.", "e.b."},
		periodsWide:            []string{"iđitbeaivet", "eahketbeaivet"},
		erasAbbreviated:        []string{"o.Kr.", "m.Kr."},
		erasNarrow:             []string{"ooá", "oá"},
		erasWide:               []string{"ovdal Kristtusa", "maŋŋel Kristtusa"},
		timezones:              map[string]string{"MST": "MST", "HEOG": "HEOG", "HEEG": "HEEG", "GMT": "Greenwich gaskka áigi", "UYT": "UYT", "BT": "BT", "AWST": "AWST", "WITA": "WITA", "AEDT": "AEDT", "ARST": "ARST", "WEZ": "oarje-Eurohpá dábálašáigi", "PDT": "PDT", "ADT": "ADT", "CLST": "CLST", "ACDT": "ACDT", "WIB": "WIB", "SRT": "SRT", "HNCU": "HNCU", "HECU": "HECU", "HAST": "HAST", "OEZ": "nuorti-Eurohpá dábálašáigi", "ART": "ART", "HNT": "HNT", "EDT": "EDT", "CHAST": "CHAST", "TMT": "TMT", "LHDT": "LHDT", "MDT": "MDT", "VET": "VET", "HENOMX": "HENOMX", "HNOG": "HNOG", "HADT": "HADT", "HNNOMX": "HNNOMX", "HAT": "HAT", "HKST": "HKST", "GYT": "GYT", "JST": "JST", "HNEG": "HNEG", "GFT": "GFT", "ECT": "ECT", "NZST": "NZST", "BOT": "BOT", "MEZ": "gaska-Eurohpá dábálašáigi", "LHST": "LHST", "IST": "IST", "HEPMX": "HEPMX", "CHADT": "CHADT", "HNPM": "HNPM", "JDT": "JDT", "SAST": "SAST", "UYST": "UYST", "MESZ": "gaska-Eurohpá geassiáigi", "NZDT": "NZDT", "CST": "CST", "ACWST": "ACWST", "ACWDT": "ACWDT", "AKDT": "AKDT", "CAT": "CAT", "ChST": "ChST", "PST": "PST", "HEPM": "HEPM", "EAT": "EAT", "COT": "COT", "CDT": "CDT", "TMST": "TMST", "EST": "EST", "OESZ": "nuorti-Eurohpá geassiáigi", "WARST": "WARST", "∅∅∅": "∅∅∅", "CLT": "CLT", "COST": "COST", "WAST": "WAST", "HKT": "HKT", "AKST": "AKST", "WESZ": "oarje-Eurohpá geassiáigi", "AWDT": "AWDT", "WIT": "WIT", "WART": "WART", "AEST": "AEST", "ACST": "ACST", "HNPMX": "HNPMX", "MYT": "MYT", "AST": "AST", "WAT": "WAT", "SGT": "SGT"},
	}
}

// Locale returns the current translators string locale
func (se *se_FI) Locale() string {
	return se.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'se_FI'
func (se *se_FI) PluralsCardinal() []locales.PluralRule {
	return se.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'se_FI'
func (se *se_FI) PluralsOrdinal() []locales.PluralRule {
	return se.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'se_FI'
func (se *se_FI) PluralsRange() []locales.PluralRule {
	return se.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'se_FI'
func (se *se_FI) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	} else if n == 2 {
		return locales.PluralRuleTwo
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'se_FI'
func (se *se_FI) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'se_FI'
func (se *se_FI) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (se *se_FI) MonthAbbreviated(month time.Month) string {
	return se.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (se *se_FI) MonthsAbbreviated() []string {
	return se.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (se *se_FI) MonthNarrow(month time.Month) string {
	return se.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (se *se_FI) MonthsNarrow() []string {
	return se.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (se *se_FI) MonthWide(month time.Month) string {
	return se.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (se *se_FI) MonthsWide() []string {
	return se.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (se *se_FI) WeekdayAbbreviated(weekday time.Weekday) string {
	return se.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (se *se_FI) WeekdaysAbbreviated() []string {
	return se.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (se *se_FI) WeekdayNarrow(weekday time.Weekday) string {
	return se.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (se *se_FI) WeekdaysNarrow() []string {
	return se.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (se *se_FI) WeekdayShort(weekday time.Weekday) string {
	return se.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (se *se_FI) WeekdaysShort() []string {
	return se.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (se *se_FI) WeekdayWide(weekday time.Weekday) string {
	return se.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (se *se_FI) WeekdaysWide() []string {
	return se.daysWide
}

// Decimal returns the decimal point of number
func (se *se_FI) Decimal() string {
	return se.decimal
}

// Group returns the group of number
func (se *se_FI) Group() string {
	return se.group
}

// Group returns the minus sign of number
func (se *se_FI) Minus() string {
	return se.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'se_FI' and handles both Whole and Real numbers based on 'v'
func (se *se_FI) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, se.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(se.group) - 1; j >= 0; j-- {
					b = append(b, se.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(se.minus) - 1; j >= 0; j-- {
			b = append(b, se.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'se_FI' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (se *se_FI) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 7
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, se.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(se.minus) - 1; j >= 0; j-- {
			b = append(b, se.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, se.percentSuffix...)

	b = append(b, se.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'se_FI'
func (se *se_FI) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := se.currencies[currency]
	l := len(s) + len(symbol) + 6 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, se.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(se.group) - 1; j >= 0; j-- {
					b = append(b, se.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		for j := len(se.minus) - 1; j >= 0; j-- {
			b = append(b, se.minus[j])
		}
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, se.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, se.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'se_FI'
// in accounting notation.
func (se *se_FI) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := se.currencies[currency]
	l := len(s) + len(symbol) + 6 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, se.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(se.group) - 1; j >= 0; j-- {
					b = append(b, se.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		for j := len(se.minus) - 1; j >= 0; j-- {
			b = append(b, se.minus[j])
		}

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, se.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, se.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, se.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'se_FI'
func (se *se_FI) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x2d}...)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2d}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'se_FI'
func (se *se_FI) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20}...)
	b = append(b, se.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'se_FI'
func (se *se_FI) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20}...)
	b = append(b, se.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'se_FI'
func (se *se_FI) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	b = append(b, []byte{0x20}...)
	b = append(b, se.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2c, 0x20}...)
	b = append(b, se.daysWide[t.Weekday()]...)

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'se_FI'
func (se *se_FI) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, se.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'se_FI'
func (se *se_FI) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, se.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, se.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'se_FI'
func (se *se_FI) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, se.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, se.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'se_FI'
func (se *se_FI) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, se.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, se.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := se.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
