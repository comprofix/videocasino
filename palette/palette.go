// Package palette is the Resurrect 64 palette by Kerrie Lake
// (lospec.com/palette-list/resurrect-64), which every colour on screen is
// taken from. The palette has no official colour names; these are the Game
// Skeleton's, in the palette's published order.
package palette

import "image/color"

// The colours, in the palette's order.
var (
	Ink        = color.RGBA{46, 34, 47, 255}    // #2e222f
	Shadow     = color.RGBA{62, 53, 70, 255}    // #3e3546
	Dusk       = color.RGBA{98, 85, 101, 255}   // #625565
	Clay       = color.RGBA{150, 108, 108, 255} // #966c6c
	Sand       = color.RGBA{171, 148, 122, 255} // #ab947a
	Mauve      = color.RGBA{105, 79, 98, 255}   // #694f62
	Lilac      = color.RGBA{127, 112, 138, 255} // #7f708a
	Fog        = color.RGBA{155, 171, 178, 255} // #9babb2
	Mist       = color.RGBA{199, 220, 208, 255} // #c7dcd0
	White      = color.RGBA{255, 255, 255, 255} // #ffffff
	Maroon     = color.RGBA{110, 39, 39, 255}   // #6e2727
	Brick      = color.RGBA{179, 56, 49, 255}   // #b33831
	Vermilion  = color.RGBA{234, 79, 54, 255}   // #ea4f36
	Coral      = color.RGBA{245, 125, 74, 255}  // #f57d4a
	Crimson    = color.RGBA{174, 35, 52, 255}   // #ae2334
	Red        = color.RGBA{232, 59, 59, 255}   // #e83b3b
	Orange     = color.RGBA{251, 107, 29, 255}  // #fb6b1d
	Amber      = color.RGBA{247, 150, 23, 255}  // #f79617
	Gold       = color.RGBA{249, 194, 43, 255}  // #f9c22b
	Wine       = color.RGBA{122, 48, 69, 255}   // #7a3045
	Rust       = color.RGBA{158, 69, 57, 255}   // #9e4539
	Copper     = color.RGBA{205, 104, 61, 255}  // #cd683d
	Caramel    = color.RGBA{230, 144, 78, 255}  // #e6904e
	Honey      = color.RGBA{251, 185, 84, 255}  // #fbb954
	Bark       = color.RGBA{76, 62, 36, 255}    // #4c3e24
	Olive      = color.RGBA{103, 102, 51, 255}  // #676633
	Moss       = color.RGBA{162, 169, 71, 255}  // #a2a947
	Chartreuse = color.RGBA{213, 224, 75, 255}  // #d5e04b
	Lemon      = color.RGBA{251, 255, 134, 255} // #fbff86
	Pine       = color.RGBA{22, 90, 76, 255}    // #165a4c
	Forest     = color.RGBA{35, 144, 99, 255}   // #239063
	Emerald    = color.RGBA{30, 188, 115, 255}  // #1ebc73
	Leaf       = color.RGBA{145, 219, 105, 255} // #91db69
	Lime       = color.RGBA{205, 223, 108, 255} // #cddf6c
	Charcoal   = color.RGBA{49, 54, 56, 255}    // #313638
	Slate      = color.RGBA{55, 78, 74, 255}    // #374e4a
	Sage       = color.RGBA{84, 126, 100, 255}  // #547e64
	Fern       = color.RGBA{146, 169, 132, 255} // #92a984
	Khaki      = color.RGBA{178, 186, 144, 255} // #b2ba90
	DeepTeal   = color.RGBA{11, 94, 101, 255}   // #0b5e65
	Teal       = color.RGBA{11, 138, 143, 255}  // #0b8a8f
	Jade       = color.RGBA{14, 175, 155, 255}  // #0eaf9b
	Aqua       = color.RGBA{48, 225, 185, 255}  // #30e1b9
	Seafoam    = color.RGBA{143, 248, 226, 255} // #8ff8e2
	Navy       = color.RGBA{50, 51, 83, 255}    // #323353
	Indigo     = color.RGBA{72, 74, 119, 255}   // #484a77
	Cobalt     = color.RGBA{77, 101, 180, 255}  // #4d65b4
	Blue       = color.RGBA{77, 155, 230, 255}  // #4d9be6
	Sky        = color.RGBA{143, 211, 255, 255} // #8fd3ff
	Plum       = color.RGBA{69, 41, 63, 255}    // #45293f
	Grape      = color.RGBA{107, 62, 117, 255}  // #6b3e75
	Violet     = color.RGBA{144, 94, 169, 255}  // #905ea9
	Lavender   = color.RGBA{168, 132, 243, 255} // #a884f3
	Orchid     = color.RGBA{234, 173, 237, 255} // #eaaded
	Berry      = color.RGBA{117, 60, 84, 255}   // #753c54
	Raspberry  = color.RGBA{162, 75, 111, 255}  // #a24b6f
	Rose       = color.RGBA{207, 101, 127, 255} // #cf657f
	Pink       = color.RGBA{237, 128, 153, 255} // #ed8099
	Mulberry   = color.RGBA{131, 28, 93, 255}   // #831c5d
	Cerise     = color.RGBA{195, 36, 84, 255}   // #c32454
	Flamingo   = color.RGBA{240, 79, 120, 255}  // #f04f78
	Salmon     = color.RGBA{246, 129, 129, 255} // #f68181
	Apricot    = color.RGBA{252, 167, 144, 255} // #fca790
	Blush      = color.RGBA{253, 203, 176, 255} // #fdcbb0
)

// All is every colour, in the palette's order.
var All = []color.RGBA{
	Ink,
	Shadow,
	Dusk,
	Clay,
	Sand,
	Mauve,
	Lilac,
	Fog,
	Mist,
	White,
	Maroon,
	Brick,
	Vermilion,
	Coral,
	Crimson,
	Red,
	Orange,
	Amber,
	Gold,
	Wine,
	Rust,
	Copper,
	Caramel,
	Honey,
	Bark,
	Olive,
	Moss,
	Chartreuse,
	Lemon,
	Pine,
	Forest,
	Emerald,
	Leaf,
	Lime,
	Charcoal,
	Slate,
	Sage,
	Fern,
	Khaki,
	DeepTeal,
	Teal,
	Jade,
	Aqua,
	Seafoam,
	Navy,
	Indigo,
	Cobalt,
	Blue,
	Sky,
	Plum,
	Grape,
	Violet,
	Lavender,
	Orchid,
	Berry,
	Raspberry,
	Rose,
	Pink,
	Mulberry,
	Cerise,
	Flamingo,
	Salmon,
	Apricot,
	Blush,
}
