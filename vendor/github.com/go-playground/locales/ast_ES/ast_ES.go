package ast_ES

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type ast_ES struct {
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

// New returns a new instance of translator for the 'ast_ES' locale
func New() locales.Translator {
	return &ast_ES{
		locale:                 "ast_ES",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         nil,
		pluralsRange:           nil,
		decimal:                ",",
		group:                  ".",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositiveSuffix: " ",
		currencyNegativeSuffix: " ",
		monthsAbbreviated:      []string{"", "xin", "feb", "mar", "abr", "may", "xun", "xnt", "ago", "set", "och", "pay", "avi"},
		monthsNarrow:           []string{"", "X", "F", "M", "A", "M", "X", "X", "A", "S", "O", "P", "A"},
		monthsWide:             []string{"", "de xineru", "de febreru", "de marzu", "d’abril", "de mayu", "de xunu", "de xunetu", "d’agostu", "de setiembre", "d’ochobre", "de payares", "d’avientu"},
		daysAbbreviated:        []string{"dom", "llu", "mar", "mié", "xue", "vie", "sáb"},
		daysNarrow:             []string{"D", "L", "M", "M", "X", "V", "S"},
		daysShort:              []string{"do", "ll", "ma", "mi", "xu", "vi", "sá"},
		daysWide:               []string{"domingu", "llunes", "martes", "miércoles", "xueves", "vienres", "sábadu"},
		periodsAbbreviated:     []string{"AM", "PM"},
		periodsNarrow:          []string{"a", "p"},
		periodsWide:            []string{"de la mañana", "de la tarde"},
		erasAbbreviated:        []string{"e.C.", "d.C."},
		erasNarrow:             []string{"", ""},
		erasWide:               []string{"enantes de Cristu", "después de Cristu"},
		timezones:              map[string]string{"CDT": "Hora braniega central norteamericana", "WARST": "Hora braniega occidental d’Arxentina", "SAST": "Hora de Sudáfrica", "COT": "Hora estándar de Colombia", "GYT": "Hora de La Guyana", "UYST": "Hora braniega del Uruguái", "HAST": "Hora estándar de Hawaii-Aleutianes", "HADT": "Hora braniega de Hawaii-Aleutianes", "LHDT": "Hora braniega de Lord Howe", "IST": "Hora estándar de la India", "WAT": "Hora estándar d’África del oeste", "AKST": "Hora estándar d’Alaska", "BOT": "Hora de Bolivia", "CST": "Hora estándar central norteamericana", "NZDT": "Hora braniega de Nueva Zelanda", "HNEG": "Hora estándar de Groenlandia oriental", "EAT": "Hora d’África del este", "CLT": "Hora estándar de Chile", "EDT": "Hora braniega del este norteamericanu", "HNPM": "Hora estándar de Saint Pierre y Miquelon", "WEZ": "Hora estándar d’Europa Occidental", "ChST": "Hora estándar de Chamorro", "MYT": "Hora de Malasia", "WART": "Hora estándar occidental d’Arxentina", "OEZ": "Hora estándar d’Europa del Este", "ARST": "Hora braniega d’Arxentina", "SGT": "Hora estándar de Singapur", "PDT": "Hora braniega del Pacíficu norteamericanu", "GMT": "Hora media de Greenwich", "SRT": "Hora del Surinam", "ACWST": "Hora estándar d’Australia central del oeste", "ACWDT": "Hora braniega d’Australia central del oeste", "MESZ": "Hora braniega d’Europa Central", "GFT": "Hora de La Guyana Francesa", "CLST": "Hora braniega de Chile", "ECT": "Hora d’Ecuador", "MEZ": "Hora estándar d’Europa Central", "HENOMX": "Hora braniega del noroeste de Méxicu", "OESZ": "Hora braniega d’Europa del Este", "CHAST": "Hora estándar de Chatham", "CHADT": "Hora braniega de Chatham", "MDT": "Hora braniega de Macáu", "HEOG": "Hora braniega de Groenlandia occidental", "COST": "Hora braniega de Colombia", "ACST": "Hora estándar d’Australia central", "PST": "Hora estándar del Pacíficu norteamericanu", "HNCU": "Hora estándar de Cuba", "AWDT": "Hora braniega d’Australia del oeste", "TMT": "Hora estándar del Turkmenistán", "WAST": "Hora braniega d’África del oeste", "HAT": "Hora braniega de Newfoundland", "ACDT": "Hora braniega d’Australia central", "∅∅∅": "hora braniega d’Acre", "UYT": "Hora estándar del Uruguái", "NZST": "Hora estándar de Nueva Zelanda", "WITA": "Hora d’Indonesia central", "HKST": "Hora braniega de Ḥong Kong", "HEPMX": "Hora braniega del Pacíficu de Méxicu", "VET": "Hora de Venezuela", "AEST": "Hora estándar d’Australia del este", "ART": "Hora estándar d’Arxentina", "HNT": "Hora estándar de Newfoundland", "EST": "Hora estándar del este norteamericanu", "TMST": "Hora braniega del Turkmenistán", "JST": "Hora estándar de Xapón", "AKDT": "Hora braniega d’Alaska", "WIB": "Hora d’Indonesia del oeste", "HECU": "Hora braniega de Cuba", "LHST": "Hora estándar de Lord Howe", "HNPMX": "Hora estándar del Pacíficu de Méxicu", "MST": "Hora estándar de Macáu", "HNOG": "Hora estándar de Groenlandia occidental", "ADT": "Hora braniega del Atlánticu", "HEPM": "Hora braniega de Saint Pierre y Miquelon", "BT": "Hora de Bután", "HNNOMX": "Hora estándar del noroeste de Méxicu", "JDT": "Hora braniega de Xapón", "AST": "Hora estándar del Atlánticu", "HKT": "Hora estándar de Ḥong Kong", "AWST": "Hora estándar d’Australia del oeste", "WIT": "Hora d’Indonesia del este", "AEDT": "Hora braniega d’Australia del este", "HEEG": "Hora braniega de Groenlandia oriental", "CAT": "Hora d’África central", "WESZ": "Hora braniega d’Europa Occidental"},
	}
}

// Locale returns the current translators string locale
func (ast *ast_ES) Locale() string {
	return ast.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'ast_ES'
func (ast *ast_ES) PluralsCardinal() []locales.PluralRule {
	return ast.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'ast_ES'
func (ast *ast_ES) PluralsOrdinal() []locales.PluralRule {
	return ast.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'ast_ES'
func (ast *ast_ES) PluralsRange() []locales.PluralRule {
	return ast.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'ast_ES'
func (ast *ast_ES) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	i := int64(n)

	if i == 1 && v == 0 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'ast_ES'
func (ast *ast_ES) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'ast_ES'
func (ast *ast_ES) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {
	return locales.PluralRuleUnknown
}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (ast *ast_ES) MonthAbbreviated(month time.Month) string {
	return ast.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (ast *ast_ES) MonthsAbbreviated() []string {
	return ast.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (ast *ast_ES) MonthNarrow(month time.Month) string {
	return ast.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (ast *ast_ES) MonthsNarrow() []string {
	return ast.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (ast *ast_ES) MonthWide(month time.Month) string {
	return ast.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (ast *ast_ES) MonthsWide() []string {
	return ast.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (ast *ast_ES) WeekdayAbbreviated(weekday time.Weekday) string {
	return ast.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (ast *ast_ES) WeekdaysAbbreviated() []string {
	return ast.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (ast *ast_ES) WeekdayNarrow(weekday time.Weekday) string {
	return ast.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (ast *ast_ES) WeekdaysNarrow() []string {
	return ast.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (ast *ast_ES) WeekdayShort(weekday time.Weekday) string {
	return ast.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (ast *ast_ES) WeekdaysShort() []string {
	return ast.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (ast *ast_ES) WeekdayWide(weekday time.Weekday) string {
	return ast.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (ast *ast_ES) WeekdaysWide() []string {
	return ast.daysWide
}

// Decimal returns the decimal point of number
func (ast *ast_ES) Decimal() string {
	return ast.decimal
}

// Group returns the group of number
func (ast *ast_ES) Group() string {
	return ast.group
}

// Group returns the minus sign of number
func (ast *ast_ES) Minus() string {
	return ast.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'ast_ES' and handles both Whole and Real numbers based on 'v'
func (ast *ast_ES) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ast.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ast.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ast.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'ast_ES' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (ast *ast_ES) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 3
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ast.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ast.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, ast.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'ast_ES'
func (ast *ast_ES) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ast.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ast.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ast.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, ast.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ast.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, ast.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'ast_ES'
// in accounting notation.
func (ast *ast_ES) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := ast.currencies[currency]
	l := len(s) + len(symbol) + 4 + 1*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, ast.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				b = append(b, ast.group[0])
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, ast.minus[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, ast.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, ast.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, ast.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'ast_ES'
func (ast *ast_ES) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2f}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2f}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'ast_ES'
func (ast *ast_ES) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ast.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'ast_ES'
func (ast *ast_ES) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ast.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20, 0x64, 0x65}...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'ast_ES'
func (ast *ast_ES) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, ast.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, ast.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20, 0x64, 0x65}...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'ast_ES'
func (ast *ast_ES) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ast.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'ast_ES'
func (ast *ast_ES) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ast.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ast.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'ast_ES'
func (ast *ast_ES) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ast.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ast.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'ast_ES'
func (ast *ast_ES) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, ast.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, ast.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := ast.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
