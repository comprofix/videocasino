# Video Casino

Casino games in one program, written in Go with
[Ebitengine](https://ebitengine.org/). A main menu chooses between them,
and they are all played from one balance of credits.

| Game | What it is |
|---|---|
| Slot Machine | A five-reel slot machine in the style of modern hold-and-respin poker machines, with free games, a link feature and four jackpots |
| Video Poker | 9/6 "full pay" Jacks or Better: you are dealt five cards, hold the ones you want, draw to replace the rest, and are paid for the hand you end up with |

This is a hobby project. It is played with pretend credits only: no real
money goes in or comes out, and the dollar amounts on screen are part of the
pretence. It is not affiliated with or endorsed by any maker of gaming
machines.

## Build and run

You need Go 1.27 or later.

### Windows

No C compiler is needed.

```
go build -ldflags -H=windowsgui -o videocasino.exe .
.\videocasino.exe
```

The `-H=windowsgui` flag stops a console window opening behind the game.

### Linux

Ebitengine needs a C compiler and some development libraries. On Debian or
Ubuntu (including WSL):

```
sudo apt install gcc libc6-dev libgl1-mesa-dev libxcursor-dev libxi-dev \
    libxinerama-dev libxrandr-dev libxxf86vm-dev libasound2-dev pkg-config
go build -o videocasino .
./videocasino
```

## The main menu

The menu lists the games in alphabetical order. Press a game's number, or
click it, to open it. Each game has a menu of its own, with how to play it,
the game itself and a note about it; the last choice there comes back to
the main menu.

| Key | Action |
|---|---|
| 1, 2, ... | Open a game; the last number exits |
| C | Insert a coin: 10,000 more credits ($100) |
| R | Reset your credits to 10,000 ($100) |
| S | Sound on or off |
| Esc | Exit |

## Credits

There is one balance, the bank, and every game plays from it. A bet comes
out of the bank and a win goes into it, so what you win in one game is
there to play with in the other.

You start with 10,000 credits ($100), at 100 credits to the dollar. The
INSERT COIN button adds 10,000 more whenever you want them, and RESET puts
the bank back to 10,000 whatever it held. Both are on the main menu and in
each game, and act on the same bank wherever they are pressed.

When the credits run out a game says so and waits. Nothing is reset for
you: insert a coin or press RESET to carry on.

## Saved game

Everything is saved after every change, so you can close the program at any
time and carry on later with the same credits, the same bet in each game,
and anything left unfinished: a hand that has been dealt but not drawn to,
free games still to play, or a link feature part-way through. Whether the
sound is on is kept too.

The save is one small file, `save.json`, in the folder the program is in.
Delete it to start again from scratch, jackpot pots included.

## Slot Machine

Choose a bet per line and a number of lines with the buttons under the
reels, then press SPIN. A spin costs the bet per line times the number of
lines. AUTO SPIN keeps spinning at that bet until you press it again, a
feature is won or the credits run short.

| Buttons | Choices |
|---|---|
| Bet per line (top row) | 1, 2, 5, 10 or 20 credits |
| Lines (bottom row) | 1, 5, 10, 25 or 50 lines |

The keyboard works too:

| Key | Action |
|---|---|
| Space or Enter | Spin |
| Up / Down | Change the bet per line |
| Left / Right | Change the number of lines |
| M | Play all 50 lines at the highest bet the credits cover |
| A | Auto spin on or off |
| C | Insert a coin |
| R | Reset your credits |
| S | Sound on or off |
| Esc | Back to the menu |

Each payline is a path across the five reels. Three or more of a symbol in
a row along a line, starting from the left reel, win; four or five in a row
pay more. Two dollars in a row pay too. A win on a line is multiplied by
the bet per line, and the wins on all the lines played are added together.

There are 50 paylines, listed in `games/slot/machine/machine.go`. Choosing
a number of lines shows them over the reels, and after a win each winning
line is shown in turn.

### Symbols

| Symbol | What it does |
|---|---|
| 10, J, Q, K, A | Low-paying line symbols |
| Cherry, plum, bell, dollar | Higher-paying line symbols |
| Joker | Wild: stands in for any of the symbols above and doubles the win. Reels 2 to 5 only |
| Dice | Scatter: three or more anywhere pay a prize and win free games |
| Poker chip | Six or more anywhere start the link feature |

### Free games

Three, four or five dice anywhere on the reels pay 2, 15 or 100 times the
total bet and win 6 free games. Free games cost nothing and play by
themselves at the bet that won them.

In a free game reels 2, 3 and 4 turn as one and show a single giant symbol,
which counts as that symbol in each of its nine places. The middle reels
always match, so a line wins when reel 1 shows the same symbol, and only
four or five in a row pay:

| Symbol | Four | Five |
|---|---|---|
| 10, J, Q, K | 5 | 15 |
| Ace | 5 | 20 |
| Cherry, plum | 10 | 25 |
| Bell | 10 | 50 |
| Dollar | 10 | 100 |

Three of the giant symbols are special:

| Giant symbol | Chance each game | What it does |
|---|---|---|
| Joker | 3% | Wild: every line pays for the symbol on reel 1. It does not double a win in a free game |
| Chip | 2.5% | Starts the link feature with its nine places held. It carries one prize: 5 to 100 times the total bet, or MINI, MINOR or MAJOR |
| Dice | 6% | Pays the total bet and adds 3 free games |

Reels 1 and 5 have no dice or chips in a free game, and the joker is on
reel 5 only.

### Link feature

The chips sit in stacks of three on the reels, and each one that lands
shows what it carries. Six or more anywhere lock in place and start the
link feature:

1. Everything else clears and you get three respins.
2. On each respin every empty position has the same chance of landing a
   chip, which locks too and puts the respins back to three.
3. When the respins run out, or all fifteen positions are full, every chip
   is paid.

A giant chip in a free game starts it as well, with nine positions already
held; the free games carry on afterwards.

| A chip can carry | Worth |
|---|---|
| Credits | 1 to 100 times the total bet |
| MINI | 20 times the total bet |
| MINOR | 200 times the total bet |
| MAJOR | The major pot |

The GRAND is not on a chip: it is paid for filling all fifteen positions.

### Jackpots

The four jackpots are shown across the top in dollars. MINI and MINOR
follow the bet. MAJOR and GRAND are pots that start at $5,000 and $20,000
and grow with every paid spin, by 1% and 0.5% of the bet; they pay the same
whatever the bet, and go back to their starting amounts when won. To keep
small bets from having the same shot at them as large ones, a MAJOR chip
and the chip that would complete the grid both land more often the more you
bet. Even at the smallest bet the GRAND comes up at least once in a hundred
million spins.

The pots are the Slot Machine's own. RESET only ever touches the credits:
the MAJOR and GRAND keep growing until they are won.

## Video Poker

Choose a coin value and a number of coins with the buttons under the cards,
then press DEAL. A hand costs the coin value times the number of coins.

| Buttons | Choices |
|---|---|
| Coin value | 1, 5, 10, 25 or 100 credits |
| BET ONE | One more coin, from 1 up to 5 and round again |
| BET MAX | Bets 5 coins and deals straight away |

Five cards are dealt. Click a card, or press its number, to hold it; do the
same again to let it go. Then press DRAW: the cards you did not hold are
replaced, and the hand is paid.

The keyboard works too:

| Key | Action |
|---|---|
| Space or Enter | Deal, then draw |
| 1 to 5 | Hold a card, or let it go |
| H | Hint: mark the best cards to hold |
| A | Auto hold on or off |
| Up / Down | Change the coin value |
| Left / Right | Change the number of coins |
| M | Bet 5 coins and deal |
| C | Insert a coin |
| R | Reset your credits |
| S | Sound on or off |
| Esc | Back to the menu |

### Paytable

What each hand pays for every coin bet:

| Hand | Pays |
|---|---|
| Royal flush | 250, or 800 when 5 coins are bet |
| Straight flush | 50 |
| Four of a kind | 25 |
| Full house | 9 |
| Flush | 6 |
| Straight | 4 |
| Three of a kind | 3 |
| Two pair | 2 |
| Jacks or better | 1 |

A pair of jacks, queens, kings or aces is the lowest hand that pays; a lower
pair pays nothing. The paytable across the top of the screen shows the same
prizes in credits, at the coin value you are playing, with a column for each
number of coins. The column for your bet is lit.

The royal flush pays more than three times as much for each coin when you
bet 5, so that is the bet the game pays back the most at.

### Auto hold and hints

With AUTO HOLD on, as it is to begin with, the best cards are held for you
as soon as the hand is dealt. You can still change them before you draw.

With it off, the holding is up to you. HINT outlines the best cards without
holding them, and says what the hand is then worth on average.

"Best" here is exact: for the hand you were dealt, every one of the 32 ways
of holding it is weighed against every draw that could follow, and the one
that pays the most on average is chosen.

The cards you draw are chosen only when you draw, so closing the program in
the middle of a hand tells you nothing about them.

## Odds

The odds of each game are worked out exactly from its rules, and the tests
check them against a great many hands and spins played out. Run the tests
to recalculate them after changing a paytable or the rules:

```
go test -v ./games/slot/machine/
go test -v ./games/videopoker/poker/
```

Every card and every spin is drawn from a cryptographic random number
generator seeded by the operating system, and seeded afresh every hundred
hands or spins. None depends on the ones before it.

### Slot Machine

At a bet of 50 credits the machine pays back about 92% of what is bet over
the long run:

| From | Share of what is bet |
|---|---|
| Line wins | 44% |
| Dice prizes and free games | 17% |
| Link chips, with MINI and MINOR | 27% |
| MAJOR and GRAND pots | 4% |

Free games are won about once in 68 spins, and about seven are played each
time. The link feature comes up about once in 82 spins, counting those a
giant chip starts. The return is the same at every bet apart from the
GRAND, which lifts it to nearly 95% at the smallest bet.

### Video Poker

Played perfectly at 5 coins, the game pays back 99.54% of what is bet over
the long run. With 1 to 4 coins it pays back 98.37%, because of the smaller
royal flush.

With perfect play at 5 coins:

| Hand | How often | Share of what is bet |
|---|---|---|
| Royal flush | Once in 40,391 hands | 1.98% |
| Straight flush | Once in 9,148 | 0.55% |
| Four of a kind | Once in 423 | 5.91% |
| Full house | Once in 87 | 10.36% |
| Flush | Once in 91 | 6.61% |
| Straight | Once in 89 | 4.49% |
| Three of a kind | Once in 13.4 | 22.33% |
| Two pair | Once in 7.7 | 25.86% |
| Jacks or better | Once in 4.7 | 21.46% |

A hand pays something a little under half the time. These figures are for
perfect play, which auto hold gives you; holding by feel pays back less.

## Layout

- `main.go` has the window, the main menu and the switching between games.
- `bank/` holds the shared credits, the settings and the save file.
- `cards/` holds the card, rank and suit types that card games share.
- `ui/` has the palette, text, shapes, buttons, card drawing and sound
  that every game uses.
- `games/slot/` has the Slot Machine's screens, with its rules in
  `games/slot/machine/`.
- `games/videopoker/` has Video Poker's screens, with its rules in
  `games/videopoker/poker/`.

The rules packages, `bank/` and `cards/` have no graphics and are covered
by tests. The art is drawn from simple shapes and the sound is generated,
so there are no asset files.

## Licence

Released under the MIT licence. See `LICENSE`.
