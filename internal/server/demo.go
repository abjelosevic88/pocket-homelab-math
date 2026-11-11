package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-math/internal/calc"
	"github.com/abjelosevic88/pocket-homelab-math/internal/store"
)

// seedDemo fills the owner's account with notebooks, saved calculations,
// kept answers and months of practice (DEMO_DATA=true): for trying it out,
// CI and the screenshots.
func (s *Server) seedDemo(ctx context.Context, uid int64) error {
	ctx = context.WithoutCancel(ctx)
	if n, _ := s.store.Notebooks(ctx, uid, store.NotebookFilter{Limit: 1}); len(n) > 0 {
		return nil
	}
	rnd := rand.New(rand.NewSource(42))
	now := time.Now()
	ago := func(days int, hour int) string {
		return time.Date(now.Year(), now.Month(), now.Day(), hour, rnd.Intn(60), 0, 0, time.Local).AddDate(0, 0, -days).UTC().Format(time.RFC3339Nano)
	}
	st := s.settingsOf(ctx, uid)
	st.Learners = []string{"Mila", "Luka"}
	st.Currency = "BAM"
	if err := s.store.SaveUserSettings(ctx, uid, st); err != nil {
		return err
	}

	mk := func(title, subject, angle string, created, changed int, pinned bool, cells []tmplCell) error {
		nb := &store.Notebook{UserID: uid, Title: title, Subject: subject, Angle: firstNonEmpty(angle, "deg"), Units: true, Pinned: pinned, CreatedAt: ago(created, 9)}
		if err := s.store.CreateNotebook(ctx, nb); err != nil {
			return err
		}
		for _, c := range (notebookTemplate{Cells: cells}).cells() {
			c.UserID, c.NotebookID, c.Position = uid, nb.ID, -1
			if err := s.store.CreateCell(ctx, &c); err != nil {
				return err
			}
		}
		at := ago(changed, 10+rnd.Intn(10))
		_, _ = s.store.DB.ExecContext(ctx, `UPDATE notebooks SET updated_at = ?, opened_at = ? WHERE id = ?`, at, at, nb.ID)
		_, _ = s.store.DB.ExecContext(ctx, `UPDATE cells SET created_at = ?, updated_at = ? WHERE notebook_id = ?`, at, at, nb.ID)
		_, err := s.recompute(ctx, uid, nb)
		return err
	}

	// fill-ups over a year, for the car notebook
	var dates, odo, litres, cost []string
	km := 84210.0
	for d := 360; d >= 0; d -= 9 + rnd.Intn(8) {
		dist := 520 + rnd.Float64()*160
		km += dist
		l := dist * (5.9 + rnd.Float64()*1.2) / 100
		price := 2.55 + 0.25*math.Sin(float64(d)/60) + rnd.Float64()*0.08
		dates = append(dates, now.AddDate(0, 0, -d).Format("2006-01-02"))
		odo = append(odo, fnum(km, 0))
		litres = append(litres, fnum(l, 2))
		cost = append(cost, fnum(l*price, 2))
	}
	carData, _ := json.Marshal(map[string]any{"columns": []column{{"Date", dates}, {"Odometer (km)", odo}, {"Fuel (L)", litres}, {"Cost (BAM)", cost}}, "chart": "line", "x": 0, "y": 3, "source": "garage"})

	// power use by month
	var months, kwh, bills []string
	for i := 17; i >= 0; i-- {
		t := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -i, 0)
		use := 290 + 140*math.Cos(float64(t.Month()-1)/12*2*math.Pi) + rnd.Float64()*30
		months = append(months, t.Format("2006-01"))
		kwh = append(kwh, fnum(use, 0))
		bills = append(bills, fnum(use*0.19+9.5, 2))
	}
	powerData, _ := json.Marshal(map[string]any{"columns": []column{{"Month", months}, {"Use (kWh)", kwh}, {"Bill (BAM)", bills}}, "chart": "bar", "x": 0, "y": 1, "source": "home"})

	// heights of a class, for averages
	var names, heights []string
	for i, n := range strings.Fields("Ana Boris Cvijeta Dino Ema Filip Goran Hana Ivan Jana Kenan Lana Marko Nina Ognjen Petra Rade Sara Tarik Una") {
		names = append(names, n)
		heights = append(heights, fnum(138+float64(i%7)*2.1+rnd.NormFloat64()*4.5, 1))
	}
	classData, _ := json.Marshal(map[string]any{"columns": []column{{"Pupil", names}, {"Height (cm)", heights}}, "chart": "histogram", "x": 0, "y": 1})

	notebooks := []struct {
		title, subject, angle string
		created, changed      int
		pinned                bool
		cells                 []tmplCell
	}{
		{"Monthly budget", "Home", "", 120, 1, true, []tmplCell{
			{Kind: "text", Body: "# November budget\nWhat comes in, what goes out. `total` adds up the lines above it."},
			{Kind: "calc", Title: "Coming in", Body: "Salary: 2450 BAM\nRent from the flat: 600 BAM\nincome = total"},
			{Kind: "calc", Title: "Going out", Body: "Loan: 612 BAM\nFood: 780 BAM\nPower: 96 BAM\nWater and rubbish: 48 BAM\nPhones and internet: 85 BAM\nCar (fuel, insurance): 310 BAM\nKids (school, sport): 220 BAM\nOther: 250 BAM\nspent = total"},
			{Kind: "calc", Title: "What is left", Body: "left = income - spent\nleft / income to %\nIn euros: left in EUR\nSaved in a year: left * 12"},
		}},
		{"Trip to Dubrovnik", "Travel", "", 46, 12, false, []tmplCell{
			{Kind: "calc", Title: "Driving", Body: "distance = 2 * 270 km\nuse = 6.4 L/100km\nfuel_price = 1.52 EUR/L\nfuel = distance * use * fuel_price\ntolls = 2 * 14.5 EUR\nparking = 4 * 20 EUR"},
			{Kind: "calc", Title: "Staying", Body: "nights = 4\nroom = 96 EUR\nstay = nights * room\nfood = nights * 4 * 18 EUR\nboat_trip = 4 * 35 EUR"},
			{Kind: "calc", Title: "Altogether", Body: "all = fuel + tolls + parking + stay + food + boat_trip\nper person: all / 4\nall in BAM\nDays until we go: 2026-12-27 - today"},
		}},
		{"Kitchen renovation", "Home", "", 80, 5, true, []tmplCell{
			{Kind: "text", Body: "## Kitchen\nFloor 4.1 × 3.2 m, walls 2.6 m high. One door (0.9 × 2.05 m) and one window (1.2 × 1.4 m)."},
			{Kind: "calc", Title: "Sizes", Body: "length = 4.1 m\nwidth = 3.2 m\nheight = 2.6 m\nfloor = length * width\nwalls = 2 (length + width) height - 0.9 m * 2.05 m - 1.2 m * 1.4 m"},
			{Kind: "calc", Title: "Tiles and paint", Body: "tiles = floor + 12%\ntile_boxes = ceil(tiles / 1.44 m²)\npaint = walls * 2 / (11 m²/L)\ntins = ceil(paint / 2.5 L)\ncost = tile_boxes * 54 BAM + tins * 38 BAM + 420 BAM"},
			{Kind: "calc", Title: "Worktop", Body: "worktop = 2.4 m + 1.6 m\nworktop * 0.6 m to m²\n320 BAM/m * worktop"},
		}},
		{"Homework: straight lines", "School", "deg", 21, 3, false, []tmplCell{
			{Kind: "text", Body: "## Exercise 4\nA taxi costs 3 BAM to start and 1.5 BAM per kilometre. How far can you go for 18 BAM?\n\nThe cost is **y = 1.5x + 3**, so we solve 1.5x + 3 = 18."},
			{Kind: "calc", Body: "1.5x + 3 = 18\nCheck: 1.5 * 10 + 3"},
			{Kind: "solve", Data: `{"mode":"system","equations":["2x + y = 7","x - y = 2"],"var":"x"}`, Summary: "2x + y = 7\nx - y = 2"},
			{Kind: "graph", Data: `{"exprs":[{"id":"a","text":"y = 1.5x + 3","color":"#3554a5"},{"id":"b","text":"y = 18","color":"#c2410c"},{"id":"c","text":"(10, 18)","color":"#15803d"}],"view":{"x":[-2,14],"y":[-2,22]}}`, Summary: "y = 1.5x + 3\ny = 18"},
		}},
		{"Physics: things falling", "School", "rad", 15, 2, false, []tmplCell{
			{Kind: "calc", Body: "h = 45 m\nt = sqrt(2h / gravity)\nv = gravity * t to km/h\nheight(x) = 45 - 4.905 x^2\nheight(2)"},
			{Kind: "graph", Data: `{"exprs":[{"id":"a","text":"y = 45 - 4.905x^2","color":"#3554a5"},{"id":"b","text":"y = 45 - k x^2","color":"#9333ea"},{"id":"c","text":"k = 1.62","slider":{"min":0.5,"max":12,"step":0.01}}],"view":{"x":[-1,8],"y":[-5,50]}}`, Summary: "y = 45 - 4.905x^2\ny = 45 - k x^2 (the Moon: k = 1.62)"},
		}},
		{"Car: fuel this year", "Car", "", 200, 4, false, []tmplCell{
			{Kind: "text", Body: "Fill-ups from Pocket Garage. Average use and what a kilometre costs."},
			{Kind: "data", Data: string(carData), Summary: "Date, Odometer (km), Fuel (L), Cost (BAM)"},
			{Kind: "calc", Body: "use = 6.4 L/100km\nprice = 2.62 BAM/L\nA kilometre costs: use * price to BAM/km"},
		}},
		{"Electricity", "Home", "", 300, 9, false, []tmplCell{
			{Kind: "data", Data: string(powerData), Summary: "Month, Use (kWh), Bill (BAM)"},
			{Kind: "calc", Title: "Heat pump or heater?", Body: "heater = 2 kW * 6 h * 120 * 0.19 BAM/kWh\nheat_pump = heater / 3.2\nheater - heat_pump"},
		}},
		{"Triangle constructions", "School", "deg", 9, 1, false, []tmplCell{
			{Kind: "text", Body: "Drag A, B or C — the lengths, the angle at A and the area follow."},
			{Kind: "geometry", Data: `{"objects":[{"id":"A","type":"point","x":-4,"y":-2},{"id":"B","type":"point","x":4,"y":-2},{"id":"C","type":"point","x":1,"y":4},{"id":"t","type":"polygon","of":["A","B","C"]},{"id":"a1","type":"angle","of":["B","A","C"]},{"id":"M","type":"midpoint","of":["A","B"]},{"id":"m1","type":"segment","of":["C","M"]},{"id":"c1","type":"circumcircle","of":["A","B","C"]}],"view":{"x":[-9,9],"y":[-6,8]}}`, Summary: "Triangle ABC with a median and its circumcircle"},
		}},
		{"Class heights", "School", "", 30, 6, false, []tmplCell{
			{Kind: "data", Data: string(classData), Summary: "Pupil, Height (cm)"},
			{Kind: "calc", Body: "148 cm to ft\n148 cm to in"},
		}},
		{"Mortgage options", "Money", "", 60, 14, false, []tmplCell{
			{Kind: "calc", Title: "Bank A — 20 years", Body: "a = pmt(4.2%/12, 20*12, 180000 BAM)\na_interest = a * 240 - 180000 BAM"},
			{Kind: "calc", Title: "Bank B — 25 years", Body: "b = pmt(3.9%/12, 25*12, 180000 BAM)\nb_interest = b * 300 - 180000 BAM"},
			{Kind: "calc", Title: "Difference", Body: "Each month: a - b\nInterest: b_interest - a_interest"},
			{Kind: "graph", Data: `{"exprs":[{"id":"a","text":"f(x) = 180000 (0.042/12) / (1 - (1 + 0.042/12)^(-12x))","color":"#3554a5"},{"id":"b","text":"g(x) = 180000 (0.039/12) / (1 - (1 + 0.039/12)^(-12x))","color":"#c2410c"}],"view":{"x":[5,35],"y":[0,2200]}}`, Summary: "Monthly payment by years"},
		}},
		{"Converting for the recipe", "Kitchen", "", 40, 25, false, []tmplCell{
			{Kind: "calc", Body: "2 cups to mL\n350 °F to °C\n8 oz to g\nButter: 3 tbsp to mL\nFor 6 instead of 4: 250 g * 6 / 4"},
		}},
	}
	for _, n := range notebooks {
		if err := mk(n.title, n.subject, n.angle, n.created, n.changed, n.pinned, n.cells); err != nil {
			return err
		}
	}

	// own calculations
	own := []struct {
		name, cat, about, code string
		in                     []calc.Input
		out                    []calc.Output
		pinned                 bool
	}{
		{"Bathroom tiles", "Home", "Tiles for walls and floor, with spare, in boxes.", "area = walls + floor\nwith_spare = area + spare\nboxes = ceil(with_spare / box)",
			[]calc.Input{in("walls", "Walls to tile", "m²", "18"), in("floor", "Floor", "m²", "4.5"), pct("spare", "Spare", "10"), in("box", "One box covers", "m²", "1.44")},
			[]calc.Output{out("Boxes", "boxes", "", 0, true), out("Tiles with spare", "with_spare", "m²", 2, false)}, true},
		{"Freelance day rate", "Money", "What to charge per day to take home a salary.", "year = salary * 12 + costs\ndays = 220 - holidays\nrate = year / days / (1 - tax)",
			[]calc.Input{in("salary", "Salary you want", "BAM", "3500"), in("costs", "Costs a year", "BAM", "6000"), in("holidays", "Days off", "", "25"), pct("tax", "Tax and contributions", "30")},
			[]calc.Output{out("Day rate", "rate", "BAM", 0, true), out("Hour rate (8 h)", "rate / 8", "BAM", 2, false)}, true},
		{"Pool water", "Home", "Water in the pool and chlorine for a shock.", "water = length * width * depth\nchlorine = water / 1 m³ * 10 g",
			[]calc.Input{in("length", "Length", "m", "6"), in("width", "Width", "m", "3"), in("depth", "Average depth", "m", "1.3")},
			[]calc.Output{out("Water", "water", "m³", 1, true), out("In litres", "water", "L", 0, false), out("Chlorine for a shock", "chlorine", "g", 0, false)}, false},
	}
	for i, o := range own {
		ib, _ := json.Marshal(o.in)
		ob, _ := json.Marshal(o.out)
		f := &store.Formula{UserID: uid, Name: o.name, Category: o.cat, About: o.about, Code: o.code, Inputs: ib, Outputs: ob, Pinned: o.pinned, CreatedAt: ago(150-i*30, 20)}
		if err := s.store.CreateFormula(ctx, f); err != nil {
			return err
		}
		if i == 0 {
			vals := map[string]string{"walls": "21", "floor": "5.2"}
			res := s.runFormula(ctx, uid, f.Code, o.in, o.out, vals)
			id := f.ID
			_ = s.store.CreateRun(ctx, &store.Run{UserID: uid, FormulaID: &id, FormulaName: f.Name, Title: "Upstairs bathroom", Inputs: mustJSON(vals), Outputs: mustJSON(res.Outputs), Main: mainAnswer(res), CreatedAt: ago(33, 19)})
		}
	}

	// kept answers of the built-ins, over months
	keep := func(key, title string, days int, vals map[string]string) {
		b, ok := libraryByKey(key)
		if !ok {
			return
		}
		res := s.runFormula(ctx, uid, b.Code, b.Inputs, b.Outputs, vals)
		if res.Error != "" {
			s.log.Warn("demo: answer not kept", "key", key, "err", res.Error)
			return
		}
		_ = s.store.CreateRun(ctx, &store.Run{UserID: uid, FormulaKey: key, FormulaName: b.Name, Title: title, Inputs: mustJSON(vals), Outputs: mustJSON(res.Outputs), Main: mainAnswer(res), CreatedAt: ago(days, 8+rnd.Intn(12))})
	}
	keep("paint", "Bedroom", 95, map[string]string{"wall_area": "34.5", "coats": "2", "coverage": "10", "tin_size": "2.5"})
	keep("paint", "Kids' room", 94, map[string]string{"wall_area": "29.2", "coats": "2", "coverage": "12", "tin_size": "2.5"})
	keep("floor", "Living room laminate", 90, map[string]string{"floor_area": "24.6", "spare": "10", "pack_area": "2.22", "pack_price": "42 BAM"})
	keep("wallpaper", "Hallway", 70, map[string]string{"walls": "11.4", "height": "2.55"})
	keep("loan", "Flat — Bank A", 61, map[string]string{"amount": "180000 BAM", "rate": "4.2", "years": "20"})
	keep("loan", "Flat — Bank B", 61, map[string]string{"amount": "180000 BAM", "rate": "3.9", "years": "25"})
	keep("savings", "Kids' savings", 55, map[string]string{"start": "4000 BAM", "monthly": "150 BAM", "rate": "3.5", "years": "12"})
	keep("vat", "New fridge", 44, map[string]string{"price": "1290 BAM", "vat": "17"})
	keep("ev_charge", "Charging at home", 38, map[string]string{"battery": "58", "start": "15", "end": "85", "charger": "7.4", "efficiency": "90", "price": "0.19 BAM/kWh"})
	keep("power_cost", "Air conditioner in summer", 120, map[string]string{"power": "1100", "use": "6", "price": "0.19 BAM/kWh"})
	for i, d := range []int{170, 140, 110, 80, 50, 20, 2} {
		keep("bmi", "", d, map[string]string{"weight": fnum(86.4-float64(i)*0.9+rnd.Float64()*0.6, 1), "height": "181"})
	}
	trips := []struct {
		title string
		km    string
		days  int
	}{{"Sarajevo", "204", 160}, {"Split", "310", 132}, {"Zagreb", "290", 101}, {"Banja Luka", "145", 75}, {"Dubrovnik", "270", 13}, {"Mostar", "92", 6}}
	for _, t := range trips {
		keep("trip_fuel", t.title, t.days, map[string]string{"distance": t.km, "ways": "There and back", "use": "6.4", "price": "2.62 BAM/L"})
	}
	keep("days_until", "Christmas", 12, map[string]string{"date": fmt.Sprintf("%d-12-25", now.Year())})
	keep("recipe_scale", "Pancakes for 6", 26, map[string]string{"amount": "250", "serves": "4", "want": "6"})
	keep("tip", "Dinner with friends", 9, map[string]string{"bill": "186 BAM", "tip": "10", "people": "5"})

	// practice: two children and yourself, over two months
	type q struct {
		Q       string `json:"q"`
		Answer  string `json:"answer"`
		Given   string `json:"given"`
		Right   bool   `json:"right"`
		Seconds int    `json:"seconds"`
	}
	learners := []struct {
		name   string
		skills []string
		level  int
		base   float64
		every  int
	}{{"Mila", []string{"tables", "times", "add", "divide"}, 2, 0.74, 1}, {"Luka", []string{"add", "subtract", "tables"}, 1, 0.68, 2}, {"", []string{"percent", "fractions", "equations"}, 4, 0.8, 5}}
	for _, l := range learners {
		for d := 60; d >= 0; d-- {
			if d%l.every != 0 || rnd.Float64() < 0.18 {
				continue
			}
			rounds := 1 + rnd.Intn(3)
			for k := 0; k < rounds; k++ {
				skill := l.skills[rnd.Intn(len(l.skills))]
				n := 10
				p := math.Min(0.98, l.base+float64(60-d)*0.003)
				var details []q
				right, secs := 0, 0
				for i := 0; i < n; i++ {
					a, b := 2+rnd.Intn(8), 2+rnd.Intn(8)
					ok := rnd.Float64() < p
					t := 2 + rnd.Intn(7)
					ans := fmt.Sprint(a * b)
					given := ans
					if !ok {
						given = fmt.Sprint(a*b + 1 + rnd.Intn(4))
					} else {
						right++
					}
					secs += t
					details = append(details, q{Q: fmt.Sprintf("%d × %d", a, b), Answer: ans, Given: given, Right: ok, Seconds: t})
				}
				day := now.AddDate(0, 0, -d)
				created := time.Date(day.Year(), day.Month(), day.Day(), 16+rnd.Intn(4), rnd.Intn(60), 0, 0, time.Local)
				_ = s.store.CreatePractice(ctx, &store.Practice{UserID: uid, Learner: l.name, Skill: skill, Level: l.level, Questions: n, Correct: right, Seconds: secs,
					Details: mustJSON(details), Day: created.Format("2006-01-02"), CreatedAt: created.UTC().Format(time.RFC3339Nano)})
			}
		}
	}
	s.log.Info("demo data ready", "user", uid)
	return nil
}
