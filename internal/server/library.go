package server

import (
	"encoding/json"

	"github.com/abjelosevic88/pocket-homelab-math/internal/calc"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// builtinFormula is a ready-made calculation everyone has.
type builtinFormula struct {
	Key      string
	Name     string
	About    string
	Category string
	Icon     string
	Code     string
	Inputs   []calc.Input
	Outputs  []calc.Output
}

func dec(n int) *int { return &n }

func in(name, label, unit, def string) calc.Input {
	return calc.Input{Name: name, Label: label, Unit: unit, Default: def}
}

func pct(name, label, def string) calc.Input {
	return calc.Input{Name: name, Label: label, Default: def, Kind: "percent"}
}

func from(i calc.Input, src, help string) calc.Input {
	i.From, i.Help = src, help
	return i
}

func out(label, expr, unit string, d int, main bool) calc.Output {
	o := calc.Output{Label: label, Expr: expr, Unit: unit, Main: main}
	if d >= 0 {
		o.Decimals = dec(d)
	}
	return o
}

// Categories in the order they are shown.
var libraryCategories = []string{"Home", "Money", "Car & travel", "Health", "Kitchen", "Dates", "School", "Science"}

var library = []builtinFormula{
	// ---- home ----
	{Key: "paint", Name: "Paint for walls", Category: "Home", Icon: "paint",
		About: "How much paint to buy for the walls of a room, and how many tins.",
		Code:  "litres = wall_area * coats / coverage\ntins = ceil(litres / tin_size)",
		Inputs: []calc.Input{
			from(in("wall_area", "Wall to paint", "m²", "30"), "rooms:wall", "Walls without doors and windows. Pocket Rooms knows it for each room."),
			in("coats", "Coats", "", "2"),
			from(in("coverage", "One litre covers", "m²/L", "10"), "", "It says on the tin, usually 8–12 m² per litre."),
			in("tin_size", "Tin size", "L", "2.5"),
		},
		Outputs: []calc.Output{out("Paint", "litres", "L", 1, true), out("Tins to buy", "tins", "", 0, false)}},
	{Key: "floor", Name: "Floor tiles or laminate", Category: "Home", Icon: "grid",
		About: "Packs of flooring for a room, with some spare for cuts.",
		Code:  "need = floor_area + spare\npacks = ceil(need / pack_area)\ncost = packs * pack_price",
		Inputs: []calc.Input{
			from(in("floor_area", "Floor", "m²", "18"), "rooms:floor", "Pocket Rooms knows the floor of each room."),
			pct("spare", "Extra for cuts", "10"),
			in("pack_area", "One pack covers", "m²", "2.2"),
			in("pack_price", "Price of a pack", "EUR", "25"),
		},
		Outputs: []calc.Output{out("Packs to buy", "packs", "", 0, true), out("Floor with spare", "need", "m²", 2, false), out("Cost", "cost", "", 2, false)}},
	{Key: "wallpaper", Name: "Wallpaper rolls", Category: "Home", Icon: "scroll",
		About: "Rolls of wallpaper for a room, counted in full-height strips.",
		Code:  "strips = ceil(walls / roll_width)\nper_roll = floor(roll_length / (height + 10 cm))\nrolls = ceil(strips / per_roll)",
		Inputs: []calc.Input{
			from(in("walls", "Length of the walls", "m", "14"), "rooms:perimeter", "All the way round the room."),
			from(in("height", "Room height", "m", "2.5"), "rooms:height", ""),
			in("roll_width", "Roll width", "cm", "53"),
			in("roll_length", "Roll length", "m", "10.05"),
		},
		Outputs: []calc.Output{out("Rolls to buy", "rolls", "", 0, true), out("Strips", "strips", "", 0, false), out("Strips per roll", "per_roll", "", 0, false)}},
	{Key: "skirting", Name: "Skirting boards", Category: "Home", Icon: "ruler",
		About: "Lengths of skirting board for a room, with 10% for cuts.",
		Code:  "pieces = ceil(length * 1.1 / piece)",
		Inputs: []calc.Input{
			from(in("length", "Wall length without doors", "m", "13"), "rooms:skirting", "Pocket Rooms takes the doors off."),
			in("piece", "One board is", "m", "2.4"),
		},
		Outputs: []calc.Output{out("Boards to buy", "pieces", "", 0, true)}},
	{Key: "concrete", Name: "Concrete for a slab", Category: "Home", Icon: "box",
		About: "Concrete for a slab or footing, and how many bags of ready mix.",
		Code:  "volume = length * width * depth\nbags = ceil(volume / bag_yield)",
		Inputs: []calc.Input{
			in("length", "Length", "m", "3"), in("width", "Width", "m", "2"), in("depth", "Depth", "cm", "10"),
			from(in("bag_yield", "One bag makes", "L", "12"), "", "A 25 kg bag makes about 12 litres."),
		},
		Outputs: []calc.Output{out("Concrete", "volume", "m³", 3, true), out("Bags", "bags", "", 0, false)}},
	{Key: "power_cost", Name: "What a device costs to run", Category: "Home", Icon: "bolt",
		About: "Electricity for a device you use every day: per day, month and year.",
		Code:  "energy = power * use\nday = energy * price",
		Inputs: []calc.Input{
			in("power", "Power", "W", "2000"), in("use", "Used each day", "h", "1.5"),
			from(in("price", "Electricity price", "EUR/kWh", "0.15"), "home:price", "On your bill, or in Pocket Home's meter."),
		},
		Outputs: []calc.Output{out("A year", "day * 365", "", 2, true), out("A month", "day * 365 / 12", "", 2, false), out("A day", "day", "", 2, false), out("Energy a year", "energy * 365", "kWh", 0, false)}},
	{Key: "heating_area", Name: "Radiator size for a room", Category: "Home", Icon: "flame",
		About: "A rough heat need for a room by its size and how well it keeps warm.",
		Code:  "need = floor_area * height * per_m3",
		Inputs: []calc.Input{
			from(in("floor_area", "Floor", "m²", "16"), "rooms:floor", ""),
			from(in("height", "Room height", "m", "2.6"), "rooms:height", ""),
			{Name: "per_m3", Label: "The house", Kind: "choice", Options: "New, well insulated=25 W/m^3|Average=40 W/m^3|Old, draughty=60 W/m^3", Default: "Average"},
		},
		Outputs: []calc.Output{out("Heat needed", "need", "W", 0, true)}},

	// ---- money ----
	{Key: "loan", Name: "Loan or mortgage payment", Category: "Money", Icon: "bank",
		About:   "The monthly payment of a loan, and what it costs in interest altogether.",
		Code:    "months = years * 12\npayment = pmt(rate / 12, months, amount)\npaid = payment * months\ninterest = paid - amount",
		Inputs:  []calc.Input{in("amount", "Borrowed", "EUR", "150000"), pct("rate", "Yearly interest", "4.5"), in("years", "Years", "", "25")},
		Outputs: []calc.Output{out("Each month", "payment", "", 2, true), out("Paid back altogether", "paid", "", 2, false), out("Interest", "interest", "", 2, false)}},
	{Key: "savings", Name: "Savings growth", Category: "Money", Icon: "piggy",
		About: "What your savings grow to when you put something in every month.",
		Code:  "months = years * 12\nend = fv(rate / 12, months, monthly, start)\nput_in = start + monthly * months\ngrowth = end - put_in",
		Inputs: []calc.Input{
			from(in("start", "Saved now", "EUR", "5000"), "wealth:liquid", "Pocket Wealth knows your savings."),
			in("monthly", "Each month", "EUR", "200"), pct("rate", "Yearly return", "4"), in("years", "Years", "", "10"),
		},
		Outputs: []calc.Output{out("At the end", "end", "", 2, true), out("You put in", "put_in", "", 2, false), out("Growth", "growth", "", 2, false)}},
	{Key: "compound", Name: "Compound interest", Category: "Money", Icon: "chart",
		About:   "A one-off amount left to grow, with interest added a number of times a year.",
		Code:    "end = compound(start, rate, years, times)",
		Inputs:  []calc.Input{in("start", "Amount", "EUR", "10000"), pct("rate", "Yearly interest", "5"), in("years", "Years", "", "10"), in("times", "Interest added times a year", "", "12")},
		Outputs: []calc.Output{out("At the end", "end", "", 2, true), out("Interest", "end - start", "", 2, false)}},
	{Key: "change", Name: "Percentage change", Category: "Money", Icon: "percent",
		About:   "How much something went up or down, in percent.",
		Code:    "change = (new - old) / old",
		Inputs:  []calc.Input{in("old", "Before", "", "80"), in("new", "Now", "", "92")},
		Outputs: []calc.Output{out("Change", "change to %", "", 2, true), out("Difference", "new - old", "", 2, false)}},
	{Key: "vat", Name: "VAT added or taken off", Category: "Money", Icon: "receipt",
		About:   "A price with VAT, and the price without VAT when it is already in.",
		Code:    "with_vat = price + vat\nwithout_vat = price / (1 + vat)",
		Inputs:  []calc.Input{in("price", "Price", "EUR", "100"), pct("vat", "VAT", "17")},
		Outputs: []calc.Output{out("With VAT added", "with_vat", "", 2, true), out("Without the VAT in it", "without_vat", "", 2, false), out("The VAT in it", "price - without_vat", "", 2, false)}},
	{Key: "discount", Name: "Sale price", Category: "Money", Icon: "tag",
		About:   "What something costs in a sale, and what you save.",
		Code:    "sale = price - discount",
		Inputs:  []calc.Input{in("price", "Price", "EUR", "79.99"), pct("discount", "Discount", "25")},
		Outputs: []calc.Output{out("You pay", "sale", "", 2, true), out("You save", "price - sale", "", 2, false)}},
	{Key: "tip", Name: "Tip and split the bill", Category: "Money", Icon: "users",
		About:   "A bill with a tip, shared between people.",
		Code:    "total = bill + tip\neach = total / people",
		Inputs:  []calc.Input{in("bill", "Bill", "EUR", "86"), pct("tip", "Tip", "10"), in("people", "People", "", "4")},
		Outputs: []calc.Output{out("Each pays", "each", "", 2, true), out("Total with tip", "total", "", 2, false)}},
	{Key: "unit_price", Name: "Which pack is cheaper", Category: "Money", Icon: "scale",
		About:   "Price per kilo (or litre) of two packs, and how much cheaper the better one is.",
		Code:    "a = price_a / size_a\nb = price_b / size_b",
		Inputs:  []calc.Input{in("price_a", "Pack A costs", "EUR", "3.49"), in("size_a", "Pack A holds", "g", "500"), in("price_b", "Pack B costs", "EUR", "5.99"), in("size_b", "Pack B holds", "kg", "1")},
		Outputs: []calc.Output{out("A per kilo", "a", "EUR/kg", 2, false), out("B per kilo", "b", "EUR/kg", 2, false), out("Cheaper by", "abs(a - b) / max(a, b) to %", "", 1, true)}},
	{Key: "hourly", Name: "Hourly pay from a salary", Category: "Money", Icon: "clock",
		About:   "What a monthly salary is per hour and per year.",
		Code:    "year = monthly * 12\nhour = year / (hours * 52)",
		Inputs:  []calc.Input{in("monthly", "Each month", "EUR", "1800"), in("hours", "Hours a week", "", "40")},
		Outputs: []calc.Output{out("Per hour", "hour", "", 2, true), out("Per year", "year", "", 2, false)}},

	// ---- car & travel ----
	{Key: "trip_fuel", Name: "Fuel for a trip", Category: "Car & travel", Icon: "fuel",
		About: "How much fuel a drive takes and what it costs, there and back if you like.",
		Code:  "litres = distance * ways * use\ncost = litres * price",
		Inputs: []calc.Input{
			in("distance", "Distance", "km", "320"),
			{Name: "ways", Label: "Trip", Kind: "choice", Options: "One way=1|There and back=2", Default: "There and back"},
			from(in("use", "Your car uses", "L/100km", "6.5"), "garage:economy", "Pocket Garage knows it from your fill-ups."),
			from(in("price", "Fuel price", "EUR/L", "1.55"), "garage:price", "The last price you paid in Pocket Garage."),
		},
		Outputs: []calc.Output{out("Cost", "cost", "", 2, true), out("Fuel", "litres", "L", 1, false)}},
	{Key: "consumption", Name: "Fuel use of a car", Category: "Car & travel", Icon: "gauge",
		About:   "What your car uses, from the distance on a full tank.",
		Code:    "use = litres / distance",
		Inputs:  []calc.Input{in("distance", "Driven", "km", "612"), in("litres", "Filled up", "L", "41.3")},
		Outputs: []calc.Output{out("Uses", "use", "L/100km", 2, true), out("Goes", "use", "km/L", 2, false), out("Miles per gallon", "use", "mpg", 1, false)}},
	{Key: "travel_time", Name: "Travel time", Category: "Car & travel", Icon: "route",
		About:   "How long a journey takes at an average speed, with breaks.",
		Code:    "time = distance / speed + breaks",
		Inputs:  []calc.Input{in("distance", "Distance", "km", "450"), in("speed", "Average speed", "km/h", "85"), in("breaks", "Breaks", "min", "30")},
		Outputs: []calc.Output{out("Time", "time", "h", 2, true), out("In minutes", "time", "min", 0, false)}},
	{Key: "ev_charge", Name: "Charging an electric car", Category: "Car & travel", Icon: "plug",
		About:   "Time and cost to charge a battery from one level to another.",
		Code:    "energy = battery * (end - start) / efficiency\ntime = energy / charger\ncost = energy * price",
		Inputs:  []calc.Input{in("battery", "Battery", "kWh", "60"), pct("start", "From", "20"), pct("end", "To", "80"), in("charger", "Charger", "kW", "11"), pct("efficiency", "Charging efficiency", "90"), in("price", "Price", "EUR/kWh", "0.15")},
		Outputs: []calc.Output{out("Time", "time", "h", 1, true), out("Energy", "energy", "kWh", 1, false), out("Cost", "cost", "", 2, false)}},

	// ---- health ----
	{Key: "bmi", Name: "Body mass index", Category: "Health", Icon: "heart",
		About: "BMI from weight and height. Only a rough guide — it does not know muscle from fat.",
		Code:  "bmi = weight / height^2",
		Inputs: []calc.Input{
			from(in("weight", "Weight", "kg", "78"), "health:weight", "Your last weight in Pocket Health."),
			from(in("height", "Height", "cm", "180"), "health:height", ""),
		},
		Outputs: []calc.Output{out("BMI", "bmi", "kg/m²", 1, true), out("Healthy weight from", "18.5 kg/m² * height^2", "kg", 0, false), out("to", "24.9 kg/m² * height^2", "kg", 0, false)}},
	{Key: "calories", Name: "Calories a day", Category: "Health", Icon: "flame",
		About: "Energy you use in a day (Mifflin–St Jeor), by how active you are.",
		Code:  "rest = (10 * weight / 1 kg + 6.25 * height / 1 cm - 5 * age + sex) * 1 kcal\nday = rest * activity",
		Inputs: []calc.Input{
			from(in("weight", "Weight", "kg", "78"), "health:weight", ""),
			from(in("height", "Height", "cm", "180"), "health:height", ""),
			in("age", "Age", "", "38"),
			{Name: "sex", Label: "Body", Kind: "choice", Options: "Male=5|Female=-161", Default: "Male"},
			{Name: "activity", Label: "How active", Kind: "choice", Options: "Mostly sitting=1.2|Light exercise=1.375|Exercise 3–5 days=1.55|Hard exercise=1.725", Default: "Light exercise"},
		},
		Outputs: []calc.Output{out("A day", "day", "kcal", 0, true), out("At rest", "rest", "kcal", 0, false)}},
	{Key: "pace", Name: "Running pace", Category: "Health", Icon: "run",
		About:   "Pace and speed of a run or walk.",
		Code:    "pace = time / distance\nspeed = distance / time",
		Inputs:  []calc.Input{in("distance", "Distance", "km", "10"), in("time", "Time", "min", "55")},
		Outputs: []calc.Output{out("Pace", "pace", "min/km", 2, true), out("Speed", "speed", "km/h", 1, false)}},
	{Key: "heart_zones", Name: "Heart rate zones", Category: "Health", Icon: "pulse",
		About:   "Rough training zones from age (220 − age).",
		Code:    "top = 220 - age",
		Inputs:  []calc.Input{in("age", "Age", "", "38")},
		Outputs: []calc.Output{out("Highest", "top", "", 0, true), out("Easy (60–70%)", "round(top * 0.6)", "", 0, false), out("Steady (70–80%)", "round(top * 0.7)", "", 0, false), out("Hard (80–90%)", "round(top * 0.8)", "", 0, false)}},
	{Key: "water", Name: "Water a day", Category: "Health", Icon: "drop",
		About:   "A rough guide: about 35 mL for each kilo you weigh.",
		Code:    "need = weight / 1 kg * 35 mL",
		Inputs:  []calc.Input{from(in("weight", "Weight", "kg", "78"), "health:weight", "")},
		Outputs: []calc.Output{out("A day", "need", "L", 1, true), out("Glasses (250 mL)", "ceil(need / 250 mL)", "", 0, false)}},

	// ---- kitchen ----
	{Key: "recipe_scale", Name: "Scale a recipe", Category: "Kitchen", Icon: "chef",
		About:   "An amount in a recipe for more (or fewer) people.",
		Code:    "new = amount * want / serves",
		Inputs:  []calc.Input{in("amount", "Amount in the recipe", "g", "250"), in("serves", "Recipe serves", "", "4"), in("want", "You cook for", "", "6")},
		Outputs: []calc.Output{out("You need", "new", "", 1, true)}},
	{Key: "oven", Name: "Oven temperature", Category: "Kitchen", Icon: "thermo",
		About:   "Celsius, Fahrenheit and fan oven (20 degrees lower).",
		Code:    "fan = temp - 20 K",
		Inputs:  []calc.Input{in("temp", "Temperature", "°C", "180")},
		Outputs: []calc.Output{out("Fahrenheit", "temp", "°F", 0, true), out("Fan oven", "fan", "°C", 0, false)}},
	{Key: "baking_pan", Name: "A different cake tin", Category: "Kitchen", Icon: "cake",
		About:   "How much more or less batter a round tin of another size needs.",
		Code:    "factor = (new_tin / old_tin)^2",
		Inputs:  []calc.Input{in("old_tin", "Recipe tin", "cm", "24"), in("new_tin", "Your tin", "cm", "28")},
		Outputs: []calc.Output{out("Multiply everything by", "factor", "", 2, true)}},

	// ---- dates ----
	{Key: "days_until", Name: "Days until", Category: "Dates", Icon: "calendar",
		About:   "Days and weeks until a date.",
		Code:    "left = days(today, date)",
		Inputs:  []calc.Input{{Name: "date", Label: "Date", Kind: "date", Default: "2026-12-25"}},
		Outputs: []calc.Output{out("Days", "left", "days", 0, true), out("Weeks", "left", "weeks", 1, false)}},
	{Key: "age", Name: "Age on a day", Category: "Dates", Icon: "cake",
		About:   "How old someone is (or will be) on a day.",
		Code:    "age = floor((day - born) / 1 year)",
		Inputs:  []calc.Input{{Name: "born", Label: "Born", Kind: "date", Default: "1988-05-14"}, {Name: "day", Label: "On", Kind: "date", Default: "today"}},
		Outputs: []calc.Output{out("Years", "age", "", 0, true), out("Days lived", "day - born", "days", 0, false)}},
	{Key: "add_days", Name: "A date from now", Category: "Dates", Icon: "calendar",
		About:   "The date a number of days, weeks or months after another.",
		Code:    "then = start + later",
		Inputs:  []calc.Input{{Name: "start", Label: "From", Kind: "date", Default: "today"}, in("later", "Later by", "days", "90")},
		Outputs: []calc.Output{out("Date", "then", "", -1, true)}},

	// ---- school ----
	{Key: "pythagoras", Name: "Pythagoras", Category: "School", Icon: "triangle",
		About:   "The long side of a right-angled triangle from the two short ones.",
		Code:    "c = sqrt(a^2 + b^2)",
		Inputs:  []calc.Input{in("a", "Side a", "", "3"), in("b", "Side b", "", "4")},
		Outputs: []calc.Output{out("Long side c", "c", "", 4, true), out("Angle at a", "atan(a / b)", "", 2, false)}},
	{Key: "circle", Name: "Circle", Category: "School", Icon: "circle",
		About:   "Area and length round a circle from its radius.",
		Code:    "area = pi * r^2\nround_length = 2 * pi * r",
		Inputs:  []calc.Input{in("r", "Radius", "cm", "5")},
		Outputs: []calc.Output{out("Area", "area", "", 3, true), out("Circumference", "round_length", "", 3, false), out("Diameter", "2r", "", 3, false)}},
	{Key: "cylinder", Name: "Cylinder (tank, pot)", Category: "School", Icon: "cylinder",
		About:   "Volume of a cylinder — how much a round tank or pot holds.",
		Code:    "volume = pi * r^2 * h",
		Inputs:  []calc.Input{in("r", "Radius", "cm", "15"), in("h", "Height", "cm", "40")},
		Outputs: []calc.Output{out("Holds", "volume", "L", 2, true), out("Volume", "volume", "cm³", 0, false)}},
	{Key: "sphere", Name: "Ball (sphere)", Category: "School", Icon: "circle",
		About:   "Volume and surface of a ball.",
		Code:    "volume = 4/3 * pi * r^3\nsurface = 4 * pi * r^2",
		Inputs:  []calc.Input{in("r", "Radius", "cm", "10")},
		Outputs: []calc.Output{out("Volume", "volume", "", 2, true), out("Surface", "surface", "", 2, false)}},
	{Key: "heron", Name: "Triangle from three sides", Category: "School", Icon: "triangle",
		About:   "Area of any triangle from its three sides (Heron).",
		Code:    "s = (a + b + c) / 2\narea = sqrt(s * (s - a) * (s - b) * (s - c))",
		Inputs:  []calc.Input{in("a", "Side a", "m", "5"), in("b", "Side b", "m", "6"), in("c", "Side c", "m", "7")},
		Outputs: []calc.Output{out("Area", "area", "", 3, true), out("Perimeter", "a + b + c", "", 3, false)}},
	{Key: "quadratic", Name: "Quadratic equation", Category: "School", Icon: "parabola",
		About:   "Solves a·x² + b·x + c = 0.",
		Code:    "d = b^2 - 4 a c",
		Inputs:  []calc.Input{in("a", "a", "", "1"), in("b", "b", "", "-1"), in("c", "c", "", "-6")},
		Outputs: []calc.Output{out("Solutions", "solve(a x^2 + b x + c = 0, x)", "", -1, true), out("Discriminant", "d", "", 4, false)}},
	{Key: "slope", Name: "Slope of a ramp or roof", Category: "School", Icon: "slope",
		About:   "A slope as a percentage and an angle, from how far it rises over how far it goes.",
		Code:    "grade = rise / run",
		Inputs:  []calc.Input{in("rise", "Rises", "cm", "30"), in("run", "Over", "m", "4")},
		Outputs: []calc.Output{out("Slope", "grade to %", "", 1, true), out("Angle", "atan(grade)", "", 1, false), out("Length of the slope", "hypot(rise, run)", "m", 2, false)}},

	// ---- science ----
	{Key: "ohm", Name: "Ohm's law", Category: "Science", Icon: "bolt",
		About:   "Current and power from voltage and resistance.",
		Code:    "current = voltage / resistance\npower = voltage * current",
		Inputs:  []calc.Input{in("voltage", "Voltage", "V", "12"), in("resistance", "Resistance", "Ω", "220")},
		Outputs: []calc.Output{out("Current", "current", "mA", 2, true), out("Power", "power", "W", 3, false)}},
	{Key: "kinetic", Name: "Energy of something moving", Category: "Science", Icon: "run",
		About:   "Kinetic energy: ½·m·v².",
		Code:    "energy = 1/2 * mass * speed^2",
		Inputs:  []calc.Input{in("mass", "Mass", "kg", "1200"), in("speed", "Speed", "km/h", "50")},
		Outputs: []calc.Output{out("Energy", "energy", "kJ", 1, true)}},
	{Key: "fall", Name: "Falling from a height", Category: "Science", Icon: "drop",
		About:   "Time to fall and speed at the bottom (without air).",
		Code:    "time = sqrt(2 * height / gravity)\nspeed = gravity * time",
		Inputs:  []calc.Input{in("height", "Height", "m", "20")},
		Outputs: []calc.Output{out("Time", "time", "s", 2, true), out("Speed", "speed", "km/h", 1, false)}},
	{Key: "density", Name: "Weight from size", Category: "Science", Icon: "box",
		About: "What a block of something weighs, from its size and material.",
		Code:  "weight = length * width * height * density",
		Inputs: []calc.Input{in("length", "Length", "cm", "100"), in("width", "Width", "cm", "20"), in("height", "Thickness", "cm", "2"),
			{Name: "density", Label: "Material", Kind: "choice", Options: "Pine=500 kg/m^3|Oak=750 kg/m^3|Concrete=2400 kg/m^3|Steel=7850 kg/m^3|Water=1000 kg/m^3|Glass=2500 kg/m^3", Default: "Oak"}},
		Outputs: []calc.Output{out("Weight", "weight", "kg", 2, true)}},
}

func libraryByKey(key string) (*builtinFormula, bool) {
	for i := range library {
		if library[i].Key == key {
			return &library[i], true
		}
	}
	return nil, false
}

// asFormula shows a built-in the way saved ones are shown.
func (b *builtinFormula) asFormula() store.Formula {
	in, _ := json.Marshal(b.Inputs)
	o, _ := json.Marshal(b.Outputs)
	return store.Formula{Key: b.Key, Name: b.Name, About: b.About, Category: b.Category, Icon: b.Icon, Code: b.Code, Inputs: in, Outputs: o, Builtin: true}
}
