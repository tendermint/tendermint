package es_PA

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type es_PA struct {
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

// New returns a new instance of translator for the 'es_PA' locale
func New() locales.Translator {
	return &es_PA{
		locale:                 "es_PA",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         []locales.PluralRule{6},
		pluralsRange:           []locales.PluralRule{6},
		decimal:                ",",
		group:                  ".",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "B/.", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		percentSuffix:          " ",
		currencyPositiveSuffix: " ",
		currencyNegativeSuffix: " ",
		monthsAbbreviated:      []string{"", "ene.", "feb.", "mar.", "abr.", "may.", "jun.", "jul.", "ago.", "sept.", "oct.", "nov.", "dic."},
		monthsNarrow:           []string{"", "E", "F", "M", "A", "M", "J", "J", "A", "S", "O", "N", "D"},
		monthsWide:             []string{"", "enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"},
		daysAbbreviated:        []string{"dom.", "lun.", "mar.", "mié.", "jue.", "vie.", "sáb."},
		daysNarrow:             []string{"D", "L", "M", "X", "J", "V", "S"},
		daysShort:              []string{"DO", "LU", "MA", "MI", "JU", "VI", "SA"},
		daysWide:               []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"},
		periodsAbbreviated:     []string{"a. m.", "p. m."},
		periodsNarrow:          []string{"a. m.", "p. m."},
		periodsWide:            []string{"a. m.", "p. m."},
		erasAbbreviated:        []string{"a. C.", "d. C."},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"antes de Cristo", "después de Cristo"},
		timezones:              map[string]string{"IST": "hora estándar de la India", "COT": "hora estándar de Colombia", "COST": "hora de verano de Colombia", "EST": "hora estándar oriental", "HNPM": "hora estándar de San Pedro y Miquelón", "CDT": "hora de verano central", "WART": "hora estándar de Argentina occidental", "∅∅∅": "hora de verano de las Azores", "WAT": "hora estándar de África occidental", "HNT": "hora estándar de Terranova", "ACDT": "hora de verano de Australia central", "WIT": "hora de Indonesia oriental", "HADT": "hora de verano de Hawái-Aleutiano", "AEDT": "hora de verano de Australia oriental", "EAT": "hora de África oriental", "WAST": "hora de verano de África occidental", "AKST": "hora estándar de Alaska", "MDT": "Hora de verano de Macao", "LHST": "hora estándar de Lord Howe", "CLST": "hora de verano de Chile", "MYT": "hora de Malasia", "TMST": "hora de verano de Turkmenistán", "WARST": "hora de verano de Argentina occidental", "HEPMX": "hora de verano del Pacífico de México", "SAST": "hora de Sudáfrica", "WESZ": "hora de verano de Europa occidental", "ChST": "hora estándar de Chamorro", "CHAST": "hora estándar de Chatham", "HNOG": "hora estándar de Groenlandia occidental", "MESZ": "hora de verano de Europa central", "VET": "hora de Venezuela", "AST": "hora estándar del Atlántico", "AEST": "hora estándar de Australia oriental", "PST": "hora estándar del Pacífico", "BOT": "hora de Bolivia", "MST": "Hora estándar de Macao", "ARST": "hora de verano de Argentina", "CLT": "hora estándar de Chile", "EDT": "hora de verano oriental", "GYT": "hora de Guyana", "SGT": "hora de Singapur", "ACWDT": "hora de verano de Australia centroccidental", "HKT": "hora estándar de Hong Kong", "WEZ": "hora estándar de Europa occidental", "AWDT": "hora de verano de Australia occidental", "TMT": "hora estándar de Turkmenistán", "OESZ": "hora de verano de Europa oriental", "HAT": "hora de verano de Terranova", "HKST": "hora de verano de Hong Kong", "ACST": "hora estándar de Australia central", "HNPMX": "hora estándar del Pacífico de México", "WIB": "hora de Indonesia occidental", "WITA": "hora de Indonesia central", "ADT": "hora de verano del Atlántico", "GMT": "hora del meridiano de Greenwich", "HECU": "hora de verano de Cuba", "CST": "hora estándar central", "ACWST": "hora estándar de Australia centroccidental", "JST": "hora estándar de Japón", "HAST": "hora estándar de Hawái-Aleutiano", "OEZ": "hora estándar de Europa oriental", "ART": "hora estándar de Argentina", "HEOG": "hora de verano de Groenlandia occidental", "GFT": "hora de la Guayana Francesa", "CHADT": "hora de verano de Chatham", "HNCU": "hora estándar de Cuba", "UYST": "hora de verano de Uruguay", "AKDT": "hora de verano de Alaska", "BT": "hora de Bután", "NZDT": "hora de verano de Nueva Zelanda", "LHDT": "hora de verano de Lord Howe", "HNEG": "hora estándar de Groenlandia oriental", "HEEG": "hora de verano de Groenlandia oriental", "CAT": "hora de África central", "PDT": "hora de verano del Pacífico", "HEPM": "hora de verano de San Pedro y Miquelón", "NZST": "hora estándar de Nueva Zelanda", "HENOMX": "hora de verano del noroeste de México", "JDT": "hora de verano de Japón", "ECT": "hora de Ecuador", "AWST": "hora estándar de Australia occidental", "SRT": "hora de Surinam", "UYT": "hora estándar de Uruguay", "MEZ": "hora estándar de Europa central", "HNNOMX": "hora estándar del noroeste de México"},
	}
}

// Locale returns the current translators string locale
func (es *es_PA) Locale() string {
	return es.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'es_PA'
func (es *es_PA) PluralsCardinal() []locales.PluralRule {
	return es.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'es_PA'
func (es *es_PA) PluralsOrdinal() []locales.PluralRule {
	return es.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'es_PA'
func (es *es_PA) PluralsRange() []locales.PluralRule {
	return es.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'es_PA'
func (es *es_PA) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'es_PA'
func (es *es_PA) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'es_PA'
func (es *es_PA) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleOther
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (es *es_PA) MonthAbbreviated(month time.Month) string {
	return es.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (es *es_PA) MonthsAbbreviated() []string {
	return es.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (es *es_PA) MonthNarrow(month time.Month) string {
	return es.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (es *es_PA) MonthsNarrow() []string {
	return es.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (es *es_PA) MonthWide(month time.Month) string {
	return es.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (es *es_PA) MonthsWide() []string {
	return es.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (es *es_PA) WeekdayAbbreviated(weekday time.Weekday) string {
	return es.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (es *es_PA) WeekdaysAbbreviated() []string {
	return es.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (es *es_PA) WeekdayNarrow(weekday time.Weekday) string {
	return es.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (es *es_PA) WeekdaysNarrow() []string {
	return es.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (es *es_PA) WeekdayShort(weekday time.Weekday) string {
	return es.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (es *es_PA) WeekdaysShort() []string {
	return es.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (es *es_PA) WeekdayWide(weekday time.Weekday) string {
	return es.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (es *es_PA) WeekdaysWide() []string {
	return es.daysWide
}

// Decimal returns the decimal point of number
func (es *es_PA) Decimal() string {
	return es.decimal
}

// Group returns the group of number
func (es *es_PA) Group() string {
	return es.group
}

// Group returns the minus sign of number
func (es *es_PA) Minus() string {
	return es.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'es_PA' and handles both Whole and Real numbers based on 'v'
func (es *es_PA) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, es.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, es.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, es.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'es_PA' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (es *es_PA) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 5
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, es.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, es.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, es.percentSuffix...)

	b = append(b, es.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'es_PA'
func (es *es_PA) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := es.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, es.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, es.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, es.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, es.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, es.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'es_PA'
// in accounting notation.
func (es *es_PA) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := es.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, es.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, es.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, es.minus[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, es.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, es.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, es.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'es_PA'
func (es *es_PA) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2f}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'es_PA'
func (es *es_PA) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Month() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Month()), 10)

	b = append(b, []byte{0x2f}...)

	if t.Day() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'es_PA'
func (es *es_PA) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20, 0x64, 0x65}...)
	b = append(b, []byte{0x20}...)
	b = append(b, es.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20, 0x64, 0x65}...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'es_PA'
func (es *es_PA) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, es.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20, 0x64, 0x65}...)
	b = append(b, []byte{0x20}...)
	b = append(b, es.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20, 0x64, 0x65}...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'es_PA'
func (es *es_PA) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, es.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, es.periodsAbbreviated[0]...)
	} else {
		b = append(b, es.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'es_PA'
func (es *es_PA) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, es.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, es.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, es.periodsAbbreviated[0]...)
	} else {
		b = append(b, es.periodsAbbreviated[1]...)
	}

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'es_PA'
func (es *es_PA) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, es.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, es.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, es.periodsAbbreviated[0]...)
	} else {
		b = append(b, es.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'es_PA'
func (es *es_PA) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	h := t.Hour()

	if h > 12 {
		h -= 12
	}

	b = strconv.AppendInt(b, int64(h), 10)
	b = append(b, es.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, es.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	if t.Hour() < 12 {
		b = append(b, es.periodsAbbreviated[0]...)
	} else {
		b = append(b, es.periodsAbbreviated[1]...)
	}

	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := es.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
