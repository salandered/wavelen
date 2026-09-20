/*
Package shades is the Open Color palette: 13 hue families of 10 shades each.

Source: https://github.com/yeun/open-color, open-color.json at 2d27c3e.
Top level white and black are not used.

The license text is in attribution/LICENSE-open-color.txt.
*/
package shades

import "github.com/salandered/wavelen/internal/color"

// Family is 10 shades of one color, lightest first.
type Family struct {
	Name   string         // like "red"
	Colors []color.Common // len is 10, named like "red-0" to "red-9"
}

const ShadeCount = 10

type shade struct {
	Hex  color.Hex
	Name string
}

type family struct {
	Name   string
	Colors []shade
}

var families = [...]family{
	{Name: "gray", Colors: []shade{
		{"#f8f9fa", "gray-0"},
		{"#f1f3f5", "gray-1"},
		{"#e9ecef", "gray-2"},
		{"#dee2e6", "gray-3"},
		{"#ced4da", "gray-4"},
		{"#adb5bd", "gray-5"},
		{"#868e96", "gray-6"},
		{"#495057", "gray-7"},
		{"#343a40", "gray-8"},
		{"#212529", "gray-9"},
	}},
	{Name: "red", Colors: []shade{
		{"#fff5f5", "red-0"},
		{"#ffe3e3", "red-1"},
		{"#ffc9c9", "red-2"},
		{"#ffa8a8", "red-3"},
		{"#ff8787", "red-4"},
		{"#ff6b6b", "red-5"},
		{"#fa5252", "red-6"},
		{"#f03e3e", "red-7"},
		{"#e03131", "red-8"},
		{"#c92a2a", "red-9"},
	}},
	{Name: "pink", Colors: []shade{
		{"#fff0f6", "pink-0"},
		{"#ffdeeb", "pink-1"},
		{"#fcc2d7", "pink-2"},
		{"#faa2c1", "pink-3"},
		{"#f783ac", "pink-4"},
		{"#f06595", "pink-5"},
		{"#e64980", "pink-6"},
		{"#d6336c", "pink-7"},
		{"#c2255c", "pink-8"},
		{"#a61e4d", "pink-9"},
	}},
	{Name: "grape", Colors: []shade{
		{"#f8f0fc", "grape-0"},
		{"#f3d9fa", "grape-1"},
		{"#eebefa", "grape-2"},
		{"#e599f7", "grape-3"},
		{"#da77f2", "grape-4"},
		{"#cc5de8", "grape-5"},
		{"#be4bdb", "grape-6"},
		{"#ae3ec9", "grape-7"},
		{"#9c36b5", "grape-8"},
		{"#862e9c", "grape-9"},
	}},
	{Name: "violet", Colors: []shade{
		{"#f3f0ff", "violet-0"},
		{"#e5dbff", "violet-1"},
		{"#d0bfff", "violet-2"},
		{"#b197fc", "violet-3"},
		{"#9775fa", "violet-4"},
		{"#845ef7", "violet-5"},
		{"#7950f2", "violet-6"},
		{"#7048e8", "violet-7"},
		{"#6741d9", "violet-8"},
		{"#5f3dc4", "violet-9"},
	}},
	{Name: "indigo", Colors: []shade{
		{"#edf2ff", "indigo-0"},
		{"#dbe4ff", "indigo-1"},
		{"#bac8ff", "indigo-2"},
		{"#91a7ff", "indigo-3"},
		{"#748ffc", "indigo-4"},
		{"#5c7cfa", "indigo-5"},
		{"#4c6ef5", "indigo-6"},
		{"#4263eb", "indigo-7"},
		{"#3b5bdb", "indigo-8"},
		{"#364fc7", "indigo-9"},
	}},
	{Name: "blue", Colors: []shade{
		{"#e7f5ff", "blue-0"},
		{"#d0ebff", "blue-1"},
		{"#a5d8ff", "blue-2"},
		{"#74c0fc", "blue-3"},
		{"#4dabf7", "blue-4"},
		{"#339af0", "blue-5"},
		{"#228be6", "blue-6"},
		{"#1c7ed6", "blue-7"},
		{"#1971c2", "blue-8"},
		{"#1864ab", "blue-9"},
	}},
	{Name: "cyan", Colors: []shade{
		{"#e3fafc", "cyan-0"},
		{"#c5f6fa", "cyan-1"},
		{"#99e9f2", "cyan-2"},
		{"#66d9e8", "cyan-3"},
		{"#3bc9db", "cyan-4"},
		{"#22b8cf", "cyan-5"},
		{"#15aabf", "cyan-6"},
		{"#1098ad", "cyan-7"},
		{"#0c8599", "cyan-8"},
		{"#0b7285", "cyan-9"},
	}},
	{Name: "teal", Colors: []shade{
		{"#e6fcf5", "teal-0"},
		{"#c3fae8", "teal-1"},
		{"#96f2d7", "teal-2"},
		{"#63e6be", "teal-3"},
		{"#38d9a9", "teal-4"},
		{"#20c997", "teal-5"},
		{"#12b886", "teal-6"},
		{"#0ca678", "teal-7"},
		{"#099268", "teal-8"},
		{"#087f5b", "teal-9"},
	}},
	{Name: "green", Colors: []shade{
		{"#ebfbee", "green-0"},
		{"#d3f9d8", "green-1"},
		{"#b2f2bb", "green-2"},
		{"#8ce99a", "green-3"},
		{"#69db7c", "green-4"},
		{"#51cf66", "green-5"},
		{"#40c057", "green-6"},
		{"#37b24d", "green-7"},
		{"#2f9e44", "green-8"},
		{"#2b8a3e", "green-9"},
	}},
	{Name: "lime", Colors: []shade{
		{"#f4fce3", "lime-0"},
		{"#e9fac8", "lime-1"},
		{"#d8f5a2", "lime-2"},
		{"#c0eb75", "lime-3"},
		{"#a9e34b", "lime-4"},
		{"#94d82d", "lime-5"},
		{"#82c91e", "lime-6"},
		{"#74b816", "lime-7"},
		{"#66a80f", "lime-8"},
		{"#5c940d", "lime-9"},
	}},
	{Name: "yellow", Colors: []shade{
		{"#fff9db", "yellow-0"},
		{"#fff3bf", "yellow-1"},
		{"#ffec99", "yellow-2"},
		{"#ffe066", "yellow-3"},
		{"#ffd43b", "yellow-4"},
		{"#fcc419", "yellow-5"},
		{"#fab005", "yellow-6"},
		{"#f59f00", "yellow-7"},
		{"#f08c00", "yellow-8"},
		{"#e67700", "yellow-9"},
	}},
	{Name: "orange", Colors: []shade{
		{"#fff4e6", "orange-0"},
		{"#ffe8cc", "orange-1"},
		{"#ffd8a8", "orange-2"},
		{"#ffc078", "orange-3"},
		{"#ffa94d", "orange-4"},
		{"#ff922b", "orange-5"},
		{"#fd7e14", "orange-6"},
		{"#f76707", "orange-7"},
		{"#e8590c", "orange-8"},
		{"#d9480f", "orange-9"},
	}},
}

func Families() []Family {
	out := make([]Family, 0, len(families))
	for _, f := range families {
		row := Family{Name: f.Name, Colors: make([]color.Common, 0, len(f.Colors))}
		for _, c := range f.Colors {
			row.Colors = append(row.Colors, color.Common{Hex: c.Hex, Name: c.Name})
		}
		out = append(out, row)
	}
	return out
}
