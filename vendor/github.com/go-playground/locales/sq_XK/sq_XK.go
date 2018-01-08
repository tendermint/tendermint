package sq_XK

import (
	"math"
	"strconv"
	"time"

	"github.com/go-playground/locales"
	"github.com/go-playground/locales/currency"
)

type sq_XK struct {
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

// New returns a new instance of translator for the 'sq_XK' locale
func New() locales.Translator {
	return &sq_XK{
		locale:                 "sq_XK",
		pluralsCardinal:        []locales.PluralRule{2, 6},
		pluralsOrdinal:         []locales.PluralRule{2, 5, 6},
		pluralsRange:           []locales.PluralRule{2, 6},
		decimal:                ",",
		group:                  " ",
		minus:                  "-",
		percent:                "%",
		perMille:               "‰",
		timeSeparator:          ":",
		inifinity:              "∞",
		currencies:             []string{"ADP", "AED", "AFA", "AFN", "ALK", "ALL", "AMD", "ANG", "AOA", "AOK", "AON", "AOR", "ARA", "ARL", "ARM", "ARP", "ARS", "ATS", "AUD", "AWG", "AZM", "AZN", "BAD", "BAM", "BAN", "BBD", "BDT", "BEC", "BEF", "BEL", "BGL", "BGM", "BGN", "BGO", "BHD", "BIF", "BMD", "BND", "BOB", "BOL", "BOP", "BOV", "BRB", "BRC", "BRE", "BRL", "BRN", "BRR", "BRZ", "BSD", "BTN", "BUK", "BWP", "BYB", "BYN", "BYR", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW", "CLE", "CLF", "CLP", "CNX", "CNY", "COP", "COU", "CRC", "CSD", "CSK", "CUC", "CUP", "CVE", "CYP", "CZK", "DDM", "DEM", "DJF", "DKK", "DOP", "DZD", "ECS", "ECV", "EEK", "EGP", "ERN", "ESA", "ESB", "ESP", "ETB", "EUR", "FIM", "FJD", "FKP", "FRF", "GBP", "GEK", "GEL", "GHC", "GHS", "GIP", "GMD", "GNF", "GNS", "GQE", "GRD", "GTQ", "GWE", "GWP", "GYD", "HKD", "HNL", "HRD", "HRK", "HTG", "HUF", "IDR", "IEP", "ILP", "ILR", "ILS", "INR", "IQD", "IRR", "ISJ", "ISK", "ITL", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW", "KRH", "KRO", "KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LTL", "LTT", "LUC", "LUF", "LUL", "LVL", "LVR", "LYD", "MAD", "MAF", "MCF", "MDC", "MDL", "MGA", "MGF", "MKD", "MKN", "MLF", "MMK", "MNT", "MOP", "MRO", "MTL", "MTP", "MUR", "MVP", "MVR", "MWK", "MXN", "MXP", "MXV", "MYR", "MZE", "MZM", "MZN", "NAD", "NGN", "NIC", "NIO", "NLG", "NOK", "NPR", "NZD", "OMR", "PAB", "PEI", "PEN", "PES", "PGK", "PHP", "PKR", "PLN", "PLZ", "PTE", "PYG", "QAR", "RHD", "ROL", "RON", "RSD", "RUB", "RUR", "RWF", "SAR", "SBD", "SCR", "SDD", "SDG", "SDP", "SEK", "SGD", "SHP", "SIT", "SKK", "SLL", "SOS", "SRD", "SRG", "SSP", "STD", "SUR", "SVC", "SYP", "SZL", "THB", "TJR", "TJS", "TMM", "TMT", "TND", "TOP", "TPE", "TRL", "TRY", "TTD", "TWD", "TZS", "UAH", "UAK", "UGS", "UGX", "USD", "USN", "USS", "UYI", "UYP", "UYU", "UZS", "VEB", "VEF", "VND", "VNN", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XDR", "XEU", "XFO", "XFU", "XOF", "XPD", "XPF", "XPT", "XRE", "XSU", "XTS", "XUA", "XXX", "YDD", "YER", "YUD", "YUM", "YUN", "YUR", "ZAL", "ZAR", "ZMK", "ZMW", "ZRN", "ZRZ", "ZWD", "ZWL", "ZWR"},
		currencyPositiveSuffix: " ",
		currencyNegativePrefix: "(",
		currencyNegativeSuffix: " )",
		monthsAbbreviated:      []string{"", "jan", "shk", "mar", "pri", "maj", "qer", "kor", "gsh", "sht", "tet", "nën", "dhj"},
		monthsNarrow:           []string{"", "j", "s", "m", "p", "m", "q", "k", "g", "s", "t", "n", "d"},
		monthsWide:             []string{"", "janar", "shkurt", "mars", "prill", "maj", "qershor", "korrik", "gusht", "shtator", "tetor", "nëntor", "dhjetor"},
		daysAbbreviated:        []string{"Die", "Hën", "Mar", "Mër", "Enj", "Pre", "Sht"},
		daysNarrow:             []string{"D", "H", "M", "M", "E", "P", "S"},
		daysShort:              []string{"Die", "Hën", "Mar", "Mër", "Enj", "Pre", "Sht"},
		daysWide:               []string{"e diel", "e hënë", "e martë", "e mërkurë", "e enjte", "e premte", "e shtunë"},
		periodsAbbreviated:     []string{"e paradites", "e pasdites"},
		periodsNarrow:          []string{"e paradites", "e pasdites"},
		periodsWide:            []string{"e paradites", "e pasdites"},
		erasAbbreviated:        []string{"p.K.", "mb.K."},
		erasNarrow:             []string{"p.K.", "mb.K."},
		erasWide:               []string{"para Krishtit", "mbas Krishtit"},
		timezones:              map[string]string{"MESZ": "Ora verore e Evropës Qendrore", "NZST": "Ora standarde e Zelandës së Re", "ACDT": "Ora verore e Australisë Qendrore", "ACWDT": "Ora verore e Australisë Qendroro-Perëndimore", "JST": "Ora standarde e Japonisë", "LHST": "Ora standarde e Lord-Houit", "HENOMX": "Ora verore e Meksikës Veriperëndimore", "∅∅∅": "Ora verore e Amazonës", "BT": "Ora e Butanit", "HNPMX": "Ora standarde meksikane e Bregut të Paqësorit", "HEPMX": "Ora verore meksikane e Bregut të Paqësorit", "WIT": "Ora e Indonezisë Lindore", "OEZ": "Ora standarde e Evropës Lindore", "OESZ": "Ora verore e Evropës Lindore", "ART": "Ora standarde e Argjentinës", "SAST": "Ora standarde e Afrikës Jugore", "HNEG": "Ora standarde e Grenlandës Lindore", "EDT": "Ora verore e SHBA-së Lindore", "GMT": "Ora e Meridianit të Grinuiçit", "CHAST": "Ora standarde e Katamit", "AWDT": "Ora verore e Australisë Perëndimore", "HAST": "Ora standarde e Ishujve Hauai-Aleutian", "HADT": "Ora verore e Ishujve Hauai-Aleutian", "EAT": "Ora e Afrikës Lindore", "WAT": "Ora standarde e Afrikës Perëndimore", "TMST": "Ora verore e Turkmenistanit", "MYT": "Ora e Malajzisë", "IST": "Ora standarde e Indisë", "ARST": "Ora verore e Argjentinës", "HEOG": "Ora verore e Grenlandës Perëndimore", "HAT": "Ora verore e Njufaundlendit [Tokës së Re]", "CAT": "Ora e Afrikës Qendrore", "COST": "Ora verore e Kolumbisë", "GFT": "Ora e Guajanës Franceze", "GYT": "Ora e Guajanës", "SGT": "Ora e Singaporit", "HNT": "Ora standarde e Njufaundlendit [Tokës së Re]", "COT": "Ora standarde e Kolumbisë", "WESZ": "Ora verore e Evropës Perëndimore", "PDT": "Ora verore amerikane e Bregut të Paqësorit", "HECU": "Ora verore e Kubës", "SRT": "Ora e Surinamit", "NZDT": "Ora verore e Zelandës së Re", "LHDT": "Ora verore e Lord-Houit", "AKST": "Ora standarde e Alaskës", "AKDT": "Ora verore e Alsaskës", "AST": "Ora standarde e Atlantikut", "UYT": "Ora standarde e Uruguait", "ADT": "Ora verore e Atlantikut", "MST": "Ora standarde e Makaos", "ACWST": "Ora standarde e Australisë Qendroro-Perëndimore", "CST": "Ora standarde e SHBA-së Qendrore", "TMT": "Ora standarde e Turkmenistanit", "VET": "Ora e Venezuelës", "HNNOMX": "Ora standarde e Meksikës Veriperëndimore", "HKT": "Ora standarde e Hong-Kongut", "EST": "Ora standarde e SHBA-së Lindore", "MEZ": "Ora standarde e Evropës Qendrore", "HEEG": "Ora verore e Grenlandës Lindore", "UYST": "Ora verore e Uruguait", "WART": "Ora standarde e Argjentinës Perëndimore", "WITA": "Ora e Indonezisë Qendrore", "JDT": "Ora verore e Japonisë", "MDT": "Ora verore e Makaos", "WAST": "Ora verore e Afrikës Perëndimore", "CHADT": "Ora verore e Katamit", "CLST": "Ora verore e Kilit", "ChST": "Ora e Kamorros", "HNPM": "Ora standarde e Shën-Pier dhe Mikuelon", "HKST": "Ora verore e Hong-Kongut", "CLT": "Ora standarde e Kilit", "HNCU": "Ora standarde e Kubës", "CDT": "Ora verore e SHBA-së Qendrore", "HEPM": "Ora verore e Shën-Pier dhe Mikuelon", "BOT": "Ora e Bolivisë", "AWST": "Ora standarde e Australisë Perëndimore", "WARST": "Ora verore e Argjentinës Perëndimore", "AEST": "Ora standarde e Australisë Lindore", "AEDT": "Ora verore e Australisë Lindore", "WEZ": "Ora standarde e Evropës Perëndimore", "PST": "Ora standarde amerikane e Bregut të Paqësorit", "WIB": "Ora e Indonezisë Perëndimore", "HNOG": "Ora standarde e Grenlandës Perëndimore", "ACST": "Ora standarde e Australisë Qendrore", "ECT": "Ora e Ekuadorit"},
	}
}

// Locale returns the current translators string locale
func (sq *sq_XK) Locale() string {
	return sq.locale
}

// PluralsCardinal returns the list of cardinal plural rules associated with 'sq_XK'
func (sq *sq_XK) PluralsCardinal() []locales.PluralRule {
	return sq.pluralsCardinal
}

// PluralsOrdinal returns the list of ordinal plural rules associated with 'sq_XK'
func (sq *sq_XK) PluralsOrdinal() []locales.PluralRule {
	return sq.pluralsOrdinal
}

// PluralsRange returns the list of range plural rules associated with 'sq_XK'
func (sq *sq_XK) PluralsRange() []locales.PluralRule {
	return sq.pluralsRange
}

// CardinalPluralRule returns the cardinal PluralRule given 'num' and digits/precision of 'v' for 'sq_XK'
func (sq *sq_XK) CardinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)

	if n == 1 {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther
}

// OrdinalPluralRule returns the ordinal PluralRule given 'num' and digits/precision of 'v' for 'sq_XK'
func (sq *sq_XK) OrdinalPluralRule(num float64, v uint64) locales.PluralRule {

	n := math.Abs(num)
	nMod10 := math.Mod(n, 10)
	nMod100 := math.Mod(n, 100)

	if n == 1 {
		return locales.PluralRuleOne
	} else if nMod10 == 4 && nMod100 != 14 {
		return locales.PluralRuleMany
	}

	return locales.PluralRuleOther
}

// RangePluralRule returns the ordinal PluralRule given 'num1', 'num2' and digits/precision of 'v1' and 'v2' for 'sq_XK'
func (sq *sq_XK) RangePluralRule(num1 float64, v1 uint64, num2 float64, v2 uint64) locales.PluralRule {

	start := sq.CardinalPluralRule(num1, v1)
	end := sq.CardinalPluralRule(num2, v2)

	if start == locales.PluralRuleOne && end == locales.PluralRuleOther {
		return locales.PluralRuleOther
	} else if start == locales.PluralRuleOther && end == locales.PluralRuleOne {
		return locales.PluralRuleOne
	}

	return locales.PluralRuleOther

}

// MonthAbbreviated returns the locales abbreviated month given the 'month' provided
func (sq *sq_XK) MonthAbbreviated(month time.Month) string {
	return sq.monthsAbbreviated[month]
}

// MonthsAbbreviated returns the locales abbreviated months
func (sq *sq_XK) MonthsAbbreviated() []string {
	return sq.monthsAbbreviated[1:]
}

// MonthNarrow returns the locales narrow month given the 'month' provided
func (sq *sq_XK) MonthNarrow(month time.Month) string {
	return sq.monthsNarrow[month]
}

// MonthsNarrow returns the locales narrow months
func (sq *sq_XK) MonthsNarrow() []string {
	return sq.monthsNarrow[1:]
}

// MonthWide returns the locales wide month given the 'month' provided
func (sq *sq_XK) MonthWide(month time.Month) string {
	return sq.monthsWide[month]
}

// MonthsWide returns the locales wide months
func (sq *sq_XK) MonthsWide() []string {
	return sq.monthsWide[1:]
}

// WeekdayAbbreviated returns the locales abbreviated weekday given the 'weekday' provided
func (sq *sq_XK) WeekdayAbbreviated(weekday time.Weekday) string {
	return sq.daysAbbreviated[weekday]
}

// WeekdaysAbbreviated returns the locales abbreviated weekdays
func (sq *sq_XK) WeekdaysAbbreviated() []string {
	return sq.daysAbbreviated
}

// WeekdayNarrow returns the locales narrow weekday given the 'weekday' provided
func (sq *sq_XK) WeekdayNarrow(weekday time.Weekday) string {
	return sq.daysNarrow[weekday]
}

// WeekdaysNarrow returns the locales narrow weekdays
func (sq *sq_XK) WeekdaysNarrow() []string {
	return sq.daysNarrow
}

// WeekdayShort returns the locales short weekday given the 'weekday' provided
func (sq *sq_XK) WeekdayShort(weekday time.Weekday) string {
	return sq.daysShort[weekday]
}

// WeekdaysShort returns the locales short weekdays
func (sq *sq_XK) WeekdaysShort() []string {
	return sq.daysShort
}

// WeekdayWide returns the locales wide weekday given the 'weekday' provided
func (sq *sq_XK) WeekdayWide(weekday time.Weekday) string {
	return sq.daysWide[weekday]
}

// WeekdaysWide returns the locales wide weekdays
func (sq *sq_XK) WeekdaysWide() []string {
	return sq.daysWide
}

// Decimal returns the decimal point of number
func (sq *sq_XK) Decimal() string {
	return sq.decimal
}

// Group returns the group of number
func (sq *sq_XK) Group() string {
	return sq.group
}

// Group returns the minus sign of number
func (sq *sq_XK) Minus() string {
	return sq.minus
}

// FmtNumber returns 'num' with digits/precision of 'v' for 'sq_XK' and handles both Whole and Real numbers based on 'v'
func (sq *sq_XK) FmtNumber(num float64, v uint64) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 2 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, sq.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(sq.group) - 1; j >= 0; j-- {
					b = append(b, sq.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, sq.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// FmtPercent returns 'num' with digits/precision of 'v' for 'sq_XK' and handles both Whole and Real numbers based on 'v'
// NOTE: 'num' passed into FmtPercent is assumed to be in percent already
func (sq *sq_XK) FmtPercent(num float64, v uint64) string {
	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	l := len(s) + 3
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, sq.decimal[0])
			continue
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, sq.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	b = append(b, sq.percent...)

	return string(b)
}

// FmtCurrency returns the currency representation of 'num' with digits/precision of 'v' for 'sq_XK'
func (sq *sq_XK) FmtCurrency(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := sq.currencies[currency]
	l := len(s) + len(symbol) + 4 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, sq.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(sq.group) - 1; j >= 0; j-- {
					b = append(b, sq.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {
		b = append(b, sq.minus[0])
	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, sq.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	b = append(b, sq.currencyPositiveSuffix...)

	b = append(b, symbol...)

	return string(b)
}

// FmtAccounting returns the currency representation of 'num' with digits/precision of 'v' for 'sq_XK'
// in accounting notation.
func (sq *sq_XK) FmtAccounting(num float64, v uint64, currency currency.Type) string {

	s := strconv.FormatFloat(math.Abs(num), 'f', int(v), 64)
	symbol := sq.currencies[currency]
	l := len(s) + len(symbol) + 6 + 2*len(s[:len(s)-int(v)-1])/3
	count := 0
	inWhole := v == 0
	b := make([]byte, 0, l)

	for i := len(s) - 1; i >= 0; i-- {

		if s[i] == '.' {
			b = append(b, sq.decimal[0])
			inWhole = true
			continue
		}

		if inWhole {
			if count == 3 {
				for j := len(sq.group) - 1; j >= 0; j-- {
					b = append(b, sq.group[j])
				}
				count = 1
			} else {
				count++
			}
		}

		b = append(b, s[i])
	}

	if num < 0 {

		b = append(b, sq.currencyNegativePrefix[0])

	}

	// reverse
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	if int(v) < 2 {

		if v == 0 {
			b = append(b, sq.decimal...)
		}

		for i := 0; i < 2-int(v); i++ {
			b = append(b, '0')
		}
	}

	if num < 0 {
		b = append(b, sq.currencyNegativeSuffix...)
		b = append(b, symbol...)
	} else {

		b = append(b, sq.currencyPositiveSuffix...)
		b = append(b, symbol...)
	}

	return string(b)
}

// FmtDateShort returns the short date representation of 't' for 'sq_XK'
func (sq *sq_XK) FmtDateShort(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x2e}...)
	b = strconv.AppendInt(b, int64(t.Month()), 10)
	b = append(b, []byte{0x2e}...)

	if t.Year() > 9 {
		b = append(b, strconv.Itoa(t.Year())[2:]...)
	} else {
		b = append(b, strconv.Itoa(t.Year())[1:]...)
	}

	return string(b)
}

// FmtDateMedium returns the medium date representation of 't' for 'sq_XK'
func (sq *sq_XK) FmtDateMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, sq.monthsAbbreviated[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateLong returns the long date representation of 't' for 'sq_XK'
func (sq *sq_XK) FmtDateLong(t time.Time) string {

	b := make([]byte, 0, 32)

	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, sq.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtDateFull returns the full date representation of 't' for 'sq_XK'
func (sq *sq_XK) FmtDateFull(t time.Time) string {

	b := make([]byte, 0, 32)

	b = append(b, sq.daysWide[t.Weekday()]...)
	b = append(b, []byte{0x2c, 0x20}...)
	b = strconv.AppendInt(b, int64(t.Day()), 10)
	b = append(b, []byte{0x20}...)
	b = append(b, sq.monthsWide[t.Month()]...)
	b = append(b, []byte{0x20}...)

	if t.Year() > 0 {
		b = strconv.AppendInt(b, int64(t.Year()), 10)
	} else {
		b = strconv.AppendInt(b, int64(-t.Year()), 10)
	}

	return string(b)
}

// FmtTimeShort returns the short time representation of 't' for 'sq_XK'
func (sq *sq_XK) FmtTimeShort(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, sq.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)

	return string(b)
}

// FmtTimeMedium returns the medium time representation of 't' for 'sq_XK'
func (sq *sq_XK) FmtTimeMedium(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, sq.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, sq.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)

	return string(b)
}

// FmtTimeLong returns the long time representation of 't' for 'sq_XK'
func (sq *sq_XK) FmtTimeLong(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, sq.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, sq.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()
	b = append(b, tz...)

	return string(b)
}

// FmtTimeFull returns the full time representation of 't' for 'sq_XK'
func (sq *sq_XK) FmtTimeFull(t time.Time) string {

	b := make([]byte, 0, 32)

	if t.Hour() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Hour()), 10)
	b = append(b, sq.timeSeparator...)

	if t.Minute() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Minute()), 10)
	b = append(b, sq.timeSeparator...)

	if t.Second() < 10 {
		b = append(b, '0')
	}

	b = strconv.AppendInt(b, int64(t.Second()), 10)
	b = append(b, []byte{0x20}...)

	tz, _ := t.Zone()

	if btz, ok := sq.timezones[tz]; ok {
		b = append(b, btz...)
	} else {
		b = append(b, tz...)
	}

	return string(b)
}
