package calc

import (
	"math"
	"sort"
	"strings"
)

// Dim is a physical dimension as powers of the base quantities:
// length, mass, time, current, temperature, amount, light, information and
// money. Angles are numbers (radians), as in SI.
type Dim [9]int8

const (
	dL = iota
	dM
	dT
	dI
	dK
	dN
	dJ
	dB
	dC
)

var dimNames = [9]string{"length", "mass", "time", "current", "temperature", "amount", "light", "data", "money"}

func (d Dim) add(o Dim, k int8) Dim {
	for i := range d {
		d[i] += o[i] * k
	}
	return d
}

func (d Dim) zero() bool { return d == Dim{} }

// unitDef is one unit: how many SI base units (or euros) one of it is.
type unitDef struct {
	Sym    string  // how it is shown
	Plural string  // shown when the amount is not one (words like day → days)
	Factor float64 // SI value of one
	Dim    Dim
	Offset float64 // for °C and °F: SI = (v + Offset) * Factor
	Prefix bool    // takes SI prefixes (k, M, m, µ…)
	Kind   string  // quantity name for the converter
	Cur    string  // currency code
}

func dim(pairs ...int) Dim {
	var d Dim
	for i := 0; i+1 < len(pairs); i += 2 {
		d[pairs[i]] = int8(pairs[i+1])
	}
	return d
}

var (
	length   = dim(dL, 1)
	mass     = dim(dM, 1)
	tm       = dim(dT, 1)
	area     = dim(dL, 2)
	volume   = dim(dL, 3)
	speed    = dim(dL, 1, dT, -1)
	force    = dim(dM, 1, dL, 1, dT, -2)
	energy   = dim(dM, 1, dL, 2, dT, -2)
	power    = dim(dM, 1, dL, 2, dT, -3)
	pressure = dim(dM, 1, dL, -1, dT, -2)
	current  = dim(dI, 1)
	voltage  = dim(dM, 1, dL, 2, dT, -3, dI, -1)
	resist   = dim(dM, 1, dL, 2, dT, -3, dI, -2)
	charge   = dim(dI, 1, dT, 1)
	temp     = dim(dK, 1)
	freq     = dim(dT, -1)
	data     = dim(dB, 1)
	amount   = dim(dN, 1)
	light    = dim(dJ, 1)
	money    = dim(dC, 1)
	none     = Dim{}
)

const (
	day   = 86400.0
	year  = 365.2425 * day
	month = year / 12
	usGal = 3.785411784e-3
)

// units by every name they answer to. The first name is the symbol.
var unitTable = []struct {
	names []string
	def   unitDef
}{
	// length
	{[]string{"m", "meter", "meters", "metre", "metres"}, unitDef{Sym: "m", Factor: 1, Dim: length, Prefix: true, Kind: "length"}},
	{[]string{"in", "inch", "inches", "″"}, unitDef{Sym: "in", Factor: 0.0254, Dim: length, Kind: "length"}},
	{[]string{"ft", "foot", "feet", "′"}, unitDef{Sym: "ft", Factor: 0.3048, Dim: length, Kind: "length"}},
	{[]string{"yd", "yard", "yards"}, unitDef{Sym: "yd", Factor: 0.9144, Dim: length, Kind: "length"}},
	{[]string{"mi", "mile", "miles"}, unitDef{Sym: "mi", Factor: 1609.344, Dim: length, Kind: "length"}},
	{[]string{"nmi", "nauticalmile", "nauticalmiles"}, unitDef{Sym: "nmi", Factor: 1852, Dim: length, Kind: "length"}},
	{[]string{"au"}, unitDef{Sym: "au", Factor: 149597870700, Dim: length, Kind: "length"}},
	{[]string{"ly", "lightyear", "lightyears"}, unitDef{Sym: "ly", Factor: 9460730472580800, Dim: length, Kind: "length"}},
	// mass
	{[]string{"g", "gram", "grams", "gramme", "grammes"}, unitDef{Sym: "g", Factor: 1e-3, Dim: mass, Prefix: true, Kind: "mass"}},
	{[]string{"t", "tonne", "tonnes", "ton", "tons"}, unitDef{Sym: "t", Factor: 1000, Dim: mass, Kind: "mass"}},
	{[]string{"lb", "lbs", "pound", "pounds"}, unitDef{Sym: "lb", Factor: 0.45359237, Dim: mass, Kind: "mass"}},
	{[]string{"oz", "ounce", "ounces"}, unitDef{Sym: "oz", Factor: 0.028349523125, Dim: mass, Kind: "mass"}},
	{[]string{"st", "stone", "stones"}, unitDef{Sym: "st", Factor: 6.35029318, Dim: mass, Kind: "mass"}},
	{[]string{"ct", "carat", "carats"}, unitDef{Sym: "ct", Factor: 0.0002, Dim: mass, Kind: "mass"}},
	// time
	{[]string{"s", "sec", "secs", "second", "seconds"}, unitDef{Sym: "s", Factor: 1, Dim: tm, Prefix: true, Kind: "time"}},
	{[]string{"min", "mins", "minute", "minutes"}, unitDef{Sym: "min", Factor: 60, Dim: tm, Kind: "time"}},
	{[]string{"h", "hr", "hrs", "hour", "hours"}, unitDef{Sym: "h", Factor: 3600, Dim: tm, Kind: "time"}},
	{[]string{"day", "days"}, unitDef{Sym: "day", Plural: "days", Factor: day, Dim: tm, Kind: "time"}},
	{[]string{"week", "weeks", "wk", "wks"}, unitDef{Sym: "week", Plural: "weeks", Factor: 7 * day, Dim: tm, Kind: "time"}},
	{[]string{"month", "months", "mo"}, unitDef{Sym: "month", Plural: "months", Factor: month, Dim: tm, Kind: "time"}},
	{[]string{"year", "years", "yr", "yrs"}, unitDef{Sym: "year", Plural: "years", Factor: year, Dim: tm, Kind: "time"}},
	// area
	{[]string{"ha", "hectare", "hectares"}, unitDef{Sym: "ha", Factor: 1e4, Dim: area, Kind: "area"}},
	{[]string{"acre", "acres", "ac"}, unitDef{Sym: "acre", Plural: "acres", Factor: 4046.8564224, Dim: area, Kind: "area"}},
	{[]string{"sqm"}, unitDef{Sym: "m²", Factor: 1, Dim: area, Kind: "area"}},
	{[]string{"sqft"}, unitDef{Sym: "ft²", Factor: 0.09290304, Dim: area, Kind: "area"}},
	{[]string{"sqkm"}, unitDef{Sym: "km²", Factor: 1e6, Dim: area, Kind: "area"}},
	{[]string{"dunum", "dunam", "dulum"}, unitDef{Sym: "dunum", Factor: 1000, Dim: area, Kind: "area"}},
	// volume
	{[]string{"L", "l", "liter", "liters", "litre", "litres"}, unitDef{Sym: "L", Factor: 1e-3, Dim: volume, Prefix: true, Kind: "volume"}},
	{[]string{"gal", "gallon", "gallons"}, unitDef{Sym: "gal", Factor: usGal, Dim: volume, Kind: "volume"}},
	{[]string{"qt", "quart", "quarts"}, unitDef{Sym: "qt", Factor: usGal / 4, Dim: volume, Kind: "volume"}},
	{[]string{"pt", "pint", "pints"}, unitDef{Sym: "pt", Factor: usGal / 8, Dim: volume, Kind: "volume"}},
	{[]string{"cup", "cups"}, unitDef{Sym: "cup", Plural: "cups", Factor: usGal / 16, Dim: volume, Kind: "volume"}},
	{[]string{"floz"}, unitDef{Sym: "fl oz", Factor: usGal / 128, Dim: volume, Kind: "volume"}},
	{[]string{"tbsp", "tablespoon", "tablespoons"}, unitDef{Sym: "tbsp", Factor: 15e-6, Dim: volume, Kind: "volume"}},
	{[]string{"tsp", "teaspoon", "teaspoons"}, unitDef{Sym: "tsp", Factor: 5e-6, Dim: volume, Kind: "volume"}},
	{[]string{"cc"}, unitDef{Sym: "cc", Factor: 1e-6, Dim: volume, Kind: "volume"}},
	// speed
	{[]string{"mph"}, unitDef{Sym: "mph", Factor: 0.44704, Dim: speed, Kind: "speed"}},
	{[]string{"kph", "kmh"}, unitDef{Sym: "km/h", Factor: 1 / 3.6, Dim: speed, Kind: "speed"}},
	{[]string{"kn", "knot", "knots"}, unitDef{Sym: "kn", Factor: 1852.0 / 3600, Dim: speed, Kind: "speed"}},
	{[]string{"mpg"}, unitDef{Sym: "mpg", Factor: 1609.344 / usGal, Dim: dim(dL, -2), Kind: "fuel"}},
	// force, energy, power, pressure
	{[]string{"N", "newton", "newtons"}, unitDef{Sym: "N", Factor: 1, Dim: force, Prefix: true, Kind: "force"}},
	{[]string{"lbf"}, unitDef{Sym: "lbf", Factor: 4.4482216152605, Dim: force, Kind: "force"}},
	{[]string{"J", "joule", "joules"}, unitDef{Sym: "J", Factor: 1, Dim: energy, Prefix: true, Kind: "energy"}},
	{[]string{"cal", "calorie", "calories"}, unitDef{Sym: "cal", Factor: 4.184, Dim: energy, Prefix: true, Kind: "energy"}},
	{[]string{"Wh"}, unitDef{Sym: "Wh", Factor: 3600, Dim: energy, Prefix: true, Kind: "energy"}},
	{[]string{"eV"}, unitDef{Sym: "eV", Factor: 1.602176634e-19, Dim: energy, Prefix: true, Kind: "energy"}},
	{[]string{"BTU", "btu"}, unitDef{Sym: "BTU", Factor: 1055.05585262, Dim: energy, Kind: "energy"}},
	{[]string{"W", "watt", "watts"}, unitDef{Sym: "W", Factor: 1, Dim: power, Prefix: true, Kind: "power"}},
	{[]string{"hp", "horsepower"}, unitDef{Sym: "hp", Factor: 745.69987158227, Dim: power, Kind: "power"}},
	{[]string{"PS"}, unitDef{Sym: "PS", Factor: 735.49875, Dim: power, Kind: "power"}},
	{[]string{"Pa", "pascal", "pascals"}, unitDef{Sym: "Pa", Factor: 1, Dim: pressure, Prefix: true, Kind: "pressure"}},
	{[]string{"bar"}, unitDef{Sym: "bar", Factor: 1e5, Dim: pressure, Prefix: true, Kind: "pressure"}},
	{[]string{"atm"}, unitDef{Sym: "atm", Factor: 101325, Dim: pressure, Kind: "pressure"}},
	{[]string{"psi"}, unitDef{Sym: "psi", Factor: 6894.757293168, Dim: pressure, Kind: "pressure"}},
	{[]string{"mmHg"}, unitDef{Sym: "mmHg", Factor: 133.322387415, Dim: pressure, Kind: "pressure"}},
	// electricity
	{[]string{"A", "amp", "amps", "ampere", "amperes"}, unitDef{Sym: "A", Factor: 1, Dim: current, Prefix: true, Kind: "current"}},
	{[]string{"V", "volt", "volts"}, unitDef{Sym: "V", Factor: 1, Dim: voltage, Prefix: true, Kind: "voltage"}},
	{[]string{"Ω", "ohm", "ohms"}, unitDef{Sym: "Ω", Factor: 1, Dim: resist, Prefix: true, Kind: "resistance"}},
	{[]string{"Ah"}, unitDef{Sym: "Ah", Factor: 3600, Dim: charge, Prefix: true, Kind: "charge"}},
	{[]string{"Hz", "hertz"}, unitDef{Sym: "Hz", Factor: 1, Dim: freq, Prefix: true, Kind: "frequency"}},
	{[]string{"rpm"}, unitDef{Sym: "rpm", Factor: 1.0 / 60, Dim: freq, Kind: "frequency"}},
	// temperature
	{[]string{"K", "kelvin"}, unitDef{Sym: "K", Factor: 1, Dim: temp, Kind: "temperature"}},
	{[]string{"°C", "degC", "celsius", "Celsius"}, unitDef{Sym: "°C", Factor: 1, Offset: 273.15, Dim: temp, Kind: "temperature"}},
	{[]string{"°F", "degF", "fahrenheit", "Fahrenheit"}, unitDef{Sym: "°F", Factor: 5.0 / 9, Offset: 459.67, Dim: temp, Kind: "temperature"}},
	// data
	{[]string{"bit", "bits"}, unitDef{Sym: "bit", Plural: "bits", Factor: 1, Dim: data, Prefix: true, Kind: "data"}},
	{[]string{"B", "byte", "bytes"}, unitDef{Sym: "B", Factor: 8, Dim: data, Prefix: true, Kind: "data"}},
	{[]string{"KiB"}, unitDef{Sym: "KiB", Factor: 8 * 1024, Dim: data, Kind: "data"}},
	{[]string{"MiB"}, unitDef{Sym: "MiB", Factor: 8 * 1024 * 1024, Dim: data, Kind: "data"}},
	{[]string{"GiB"}, unitDef{Sym: "GiB", Factor: 8 * 1024 * 1024 * 1024, Dim: data, Kind: "data"}},
	{[]string{"TiB"}, unitDef{Sym: "TiB", Factor: 8 * 1024 * 1024 * 1024 * 1024, Dim: data, Kind: "data"}},
	// angles (plain numbers in radians)
	{[]string{"rad", "radian", "radians"}, unitDef{Sym: "rad", Factor: 1, Dim: none, Kind: "angle"}},
	{[]string{"deg", "degree", "degrees", "°"}, unitDef{Sym: "°", Factor: math.Pi / 180, Dim: none, Kind: "angle"}},
	{[]string{"turn", "turns", "rev"}, unitDef{Sym: "turn", Plural: "turns", Factor: 2 * math.Pi, Dim: none, Kind: "angle"}},
	// amount, light
	{[]string{"mol"}, unitDef{Sym: "mol", Factor: 1, Dim: amount, Prefix: true, Kind: "amount"}},
	{[]string{"cd", "candela"}, unitDef{Sym: "cd", Factor: 1, Dim: light, Kind: "light"}},
}

var prefixes = []struct {
	p string
	f float64
}{
	{"da", 1e1}, {"Y", 1e24}, {"Z", 1e21}, {"E", 1e18}, {"P", 1e15}, {"T", 1e12}, {"G", 1e9}, {"M", 1e6}, {"k", 1e3}, {"h", 1e2},
	{"d", 1e-1}, {"c", 1e-2}, {"m", 1e-3}, {"µ", 1e-6}, {"μ", 1e-6}, {"u", 1e-6}, {"n", 1e-9}, {"p", 1e-12}, {"f", 1e-15},
}

// currency symbols and names people write; ISO codes come from the rates.
var currencyAliases = map[string]string{
	"€": "EUR", "$": "USD", "£": "GBP", "¥": "JPY", "₹": "INR", "₽": "RUB", "₺": "TRY", "₩": "KRW", "₣": "CHF",
	"KM": "BAM", "euro": "EUR", "euros": "EUR", "dollar": "USD", "dollars": "USD", "pound sterling": "GBP",
	"marka": "BAM", "marke": "BAM", "maraka": "BAM", "dinar": "RSD", "dinara": "RSD", "kn": "",
}

var unitIndex map[string]*unitDef

func init() {
	unitIndex = map[string]*unitDef{}
	for i := range unitTable {
		u := &unitTable[i].def
		for _, n := range unitTable[i].names {
			unitIndex[n] = u
		}
	}
}

// lookupUnit finds a unit by name (with an SI prefix when the unit takes
// one). Currencies are looked up separately (they need the rates).
func lookupUnit(name string) (*unitDef, bool) {
	if u, ok := unitIndex[name]; ok {
		return u, true
	}
	for _, p := range prefixes {
		if !strings.HasPrefix(name, p.p) || len(name) == len(p.p) {
			continue
		}
		base, ok := unitIndex[name[len(p.p):]]
		if !ok || !base.Prefix || base.Sym != name[len(p.p):] {
			continue
		}
		d := *base
		d.Sym = name
		d.Plural = ""
		d.Factor = base.Factor * p.f
		return &d, true
	}
	return nil, false
}

// UnitPow is one unit in a value's unit list, e.g. km¹ or h⁻¹.
type UnitPow struct {
	Name string `json:"name"`
	Pow  int    `json:"pow"`
}

// Units is the unit list a value is shown in, kept as written.
type Units []UnitPow

func (u Units) clone() Units { return append(Units(nil), u...) }

// Converter list: the kinds and their common units, for the converter page.
var ConverterKinds = []struct {
	Kind  string   `json:"kind"`
	Units []string `json:"units"`
}{
	{"length", []string{"mm", "cm", "m", "km", "in", "ft", "yd", "mi", "nmi"}},
	{"mass", []string{"mg", "g", "kg", "t", "oz", "lb", "st", "ct"}},
	{"temperature", []string{"°C", "°F", "K"}},
	{"area", []string{"cm²", "m²", "km²", "ha", "dunum", "in²", "ft²", "acre"}},
	{"volume", []string{"mL", "L", "m³", "tsp", "tbsp", "cup", "floz", "pt", "qt", "gal"}},
	{"speed", []string{"m/s", "km/h", "mph", "kn"}},
	{"time", []string{"ms", "s", "min", "h", "day", "week", "month", "year"}},
	{"energy", []string{"J", "kJ", "cal", "kcal", "Wh", "kWh", "BTU"}},
	{"power", []string{"W", "kW", "hp", "PS"}},
	{"pressure", []string{"Pa", "hPa", "kPa", "bar", "mbar", "atm", "psi", "mmHg"}},
	{"data", []string{"bit", "B", "kB", "MB", "GB", "TB", "KiB", "MiB", "GiB"}},
	{"angle", []string{"°", "rad", "turn"}},
	{"fuel", []string{"L/100km", "km/L", "mpg"}},
}

// UnitNames lists every unit name (for autocompletion), sorted.
func UnitNames() []string {
	out := make([]string, 0, len(unitIndex))
	for n := range unitIndex {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
