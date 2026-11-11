package server

import (
	"encoding/json"

	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// notebookTemplate is a ready start for a new notebook.
type notebookTemplate struct {
	Key, Name, About, Icon string
	Title, Subject, Angle  string
	Cells                  []tmplCell
}

type tmplCell struct {
	Kind, Title, Body, Data, Summary string
}

func (t notebookTemplate) cells() []store.Cell {
	out := make([]store.Cell, 0, len(t.Cells))
	for _, c := range t.Cells {
		data := c.Data
		if data == "" {
			data = "{}"
		}
		sum := c.Summary
		if c.Kind == "text" {
			sum = c.Body
		}
		out = append(out, store.Cell{Kind: c.Kind, Title: c.Title, Body: c.Body, Data: json.RawMessage(data), Summary: sum})
	}
	return out
}

var notebookTemplates = []notebookTemplate{
	{Key: "blank", Name: "Blank notebook", Icon: "calc", About: "One calculation to start from.", Title: "Untitled notebook",
		Cells: []tmplCell{{Kind: "calc"}}},
	{Key: "budget", Name: "Monthly budget", Icon: "wallet", About: "Money in, money out and what is left, with totals.", Title: "Monthly budget", Subject: "Home",
		Cells: []tmplCell{
			{Kind: "text", Body: "# Monthly budget\nWrite each amount on its own line. `total` adds up the lines above it, back to an empty line."},
			{Kind: "calc", Title: "Coming in", Body: "Salary: 1800 EUR\nSide work: 250 EUR\nincome = total"},
			{Kind: "calc", Title: "Going out", Body: "Rent: 450 EUR\nFood: 380 EUR\nPower and water: 95 EUR\nPhone and internet: 40 EUR\nCar: 160 EUR\nOther: 150 EUR\nspent = total"},
			{Kind: "calc", Title: "What is left", Body: "left = income - spent\nleft / income to %\nSaved in a year: left * 12"},
		}},
	{Key: "trip", Name: "Trip costs", Icon: "car", About: "Fuel, tolls, a place to stay — and each person's share.", Title: "Trip", Subject: "Travel",
		Cells: []tmplCell{
			{Kind: "calc", Title: "Driving", Body: "distance = 2 * 320 km\nuse = 6.5 L/100km\nfuel_price = 1.55 EUR/L\nfuel = distance * use * fuel_price\ntolls = 2 * 18 EUR"},
			{Kind: "calc", Title: "Staying", Body: "nights = 3\nroom = 75 EUR\nstay = nights * room\nfood = nights * 2 * 45 EUR"},
			{Kind: "calc", Title: "Altogether", Body: "all = fuel + tolls + stay + food\npeople = 2\neach = all / people"},
		}},
	{Key: "renovation", Name: "Doing up a room", Icon: "paint", About: "Floor, walls and paint for one room.", Title: "Doing up a room", Subject: "Home",
		Cells: []tmplCell{
			{Kind: "calc", Title: "The room", Body: "length = 4.2 m\nwidth = 3.6 m\nheight = 2.6 m\nfloor = length * width\nwalls = 2 * (length + width) * height - 2 m² - 1.8 m²"},
			{Kind: "calc", Title: "Paint and floor", Body: "paint = walls * 2 / (10 m²/L)\nceil(paint / 2.5 L)\nlaminate = floor + 10%\npacks = ceil(laminate / 2.2 m²)\ncost = packs * 25 EUR + ceil(paint / 2.5 L) * 32 EUR"},
		}},
	{Key: "homework", Name: "Homework", Icon: "school", About: "Words, sums, an equation and a graph for one exercise.", Title: "Homework", Subject: "School", Angle: "deg",
		Cells: []tmplCell{
			{Kind: "text", Body: "## Exercise\nWrite the question here."},
			{Kind: "calc", Body: ""},
			{Kind: "solve", Data: `{"mode":"equation","equations":["2x + 3 = 11"],"var":"x"}`, Summary: "2x + 3 = 11"},
			{Kind: "graph", Data: `{"exprs":[{"id":"a","text":"y = 2x + 3","color":"#3554a5"}],"view":{"x":[-10,10],"y":[-7,13]}}`, Summary: "y = 2x + 3"},
		}},
	{Key: "graph", Name: "Graph", Icon: "graph", About: "Draw functions, points and sliders.", Title: "Graph", Angle: "rad",
		Cells: []tmplCell{{Kind: "graph", Data: `{"exprs":[{"id":"a","text":"y = sin(x)","color":"#3554a5"},{"id":"b","text":"y = a x^2 + b","color":"#c2410c"},{"id":"c","text":"a = 0.5","slider":{"min":-3,"max":3,"step":0.1}},{"id":"d","text":"b = -2","slider":{"min":-5,"max":5,"step":0.5}}],"view":{"x":[-10,10],"y":[-6,6]}}`, Summary: "y = sin(x)\ny = a x^2 + b"}}},
	{Key: "geometry", Name: "Geometry", Icon: "triangle", About: "Points, lines and circles you can drag, with lengths and angles.", Title: "Geometry",
		Cells: []tmplCell{{Kind: "geometry", Data: `{"objects":[{"id":"A","type":"point","x":-3,"y":-2},{"id":"B","type":"point","x":4,"y":-2},{"id":"C","type":"point","x":1,"y":3},{"id":"t","type":"polygon","of":["A","B","C"]},{"id":"m","type":"angle","of":["B","A","C"]}],"view":{"x":[-8,8],"y":[-6,6]}}`, Summary: "Triangle ABC"}}},
	{Key: "data", Name: "Data and averages", Icon: "table", About: "A table of numbers with averages, a trend line and a chart.", Title: "Data",
		Cells: []tmplCell{{Kind: "data", Data: `{"columns":[{"name":"Month","values":["Jan","Feb","Mar","Apr","May","Jun"]},{"name":"Spent","values":["820","760","905","870","810","930"]}],"chart":"line","x":0,"y":1}`, Summary: "Month, Spent"}}},
	{Key: "loan", Name: "Loan comparison", Icon: "bank", About: "Two loans side by side: payment and total interest.", Title: "Loans compared", Subject: "Money",
		Cells: []tmplCell{
			{Kind: "calc", Title: "Bank A", Body: "a = pmt(4.2%/12, 20*12, 120000 EUR)\na_interest = a * 240 - 120000 EUR"},
			{Kind: "calc", Title: "Bank B", Body: "b = pmt(3.9%/12, 25*12, 120000 EUR)\nb_interest = b * 300 - 120000 EUR"},
			{Kind: "calc", Title: "Difference", Body: "a - b\na_interest - b_interest"},
		}},
}

func templateByKey(k string) (notebookTemplate, bool) {
	for _, t := range notebookTemplates {
		if t.Key == k {
			return t, true
		}
	}
	return notebookTemplate{}, false
}
