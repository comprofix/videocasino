# Video Casino

Casino games in one program, written in Go with
[Ebitengine](https://ebitengine.org/). A main menu chooses between them,
and they are all played from one balance of credits.

| Game | What it is |
|---|---|
| Baccarat | The player's hand against the banker's from eight decks, with no commission and bets on a tie or a pair |
| Blackjack | Six decks dealt down to a cut card, with doubling, splitting and the Perfect Pairs side bet |
| Roulette | A wheel with a single zero and the full table to bet on, from single numbers to red or black |
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

The menu lists the games in alphabetical order, with EXIT under them. Each
game has a menu of its own, with how to play it, the game itself and a note
about it; the last choice there comes back to the main menu.

Every menu works the same way. One entry is marked with a yellow bar: the
up and down arrows move the bar and Enter chooses the entry it is on. The
number beside an entry chooses it at once, and so does a click.

With four games or more the main menu is set out in two columns of four,
read down the left and then down the right. Games that do not fit go onto
further pages: the last place on a page is then NEXT PAGE, and the first
place on any page after the first is PREVIOUS PAGE. The left and right
arrows turn the page too.

| Key | Action |
|---|---|
| Up / Down | Move to another entry |
| Enter | Choose the entry you are on |
| 1, 2, ... | Choose the entry with that number |
| 0 | Exit |
| Left / Right | The page before, or the next |
| C | Insert a coin: 10,000 more credits ($100) |
| R | Reset your credits to 10,000 ($100) |
| S | Sound on or off |
| Esc | The page before; on the first page, exit |

In a game's menu Esc goes back to the main menu. A game's help and about
screens are left with Enter, Space, Esc or a click.

## Credits

There is one balance, the bank, and every game plays from it. A bet comes
out of the bank and a win goes into it, so what you win in one game is
there to play with in the others.

You start with 10,000 credits ($100), at 100 credits to the dollar. The
INSERT COIN button adds 10,000 more whenever you want them, and RESET puts
the bank back to 10,000 whatever it held. Both are on the main menu and in
each game, and act on the same bank wherever they are pressed.

When the credits run out a game says so and waits. Nothing is reset for
you: insert a coin or press RESET to carry on.

## Saved game

Everything is saved after every change, so you can close the program at any
time and carry on later with the same credits, the same bet in each game,
and anything left unfinished: a hand that has been dealt but not played
out, free games still to play, or a link feature part-way through. The
Blackjack and Baccarat shoes are kept as they stand, as are the chips on
the Baccarat and Roulette tables and whether the sound is on.

The save is one small file, `save.json`, in the folder the program is in.
Delete it to start again from scratch, jackpot pots included.

Nothing in the save tells you a card before it is dealt. The cards drawn in
Video Poker are chosen only at the draw. The Blackjack and Baccarat shoes are
kept as how many of each card are left, not as cards in an order. The
Blackjack dealer has no hidden card, and a hand of Baccarat is dealt and
settled all at once.

## Baccarat

Two hands are dealt, the player's and the banker's. You play neither: you
bet on which will win, on a tie, or on either hand starting with a pair,
then press DEAL.

An ace counts 1, a two to a nine its number, and a ten or a picture nothing.
Only the last figure of a hand counts, so a seven and an eight make 5. The
hand nearer 9 wins.

Each hand is dealt two cards. If either makes 8 or 9, a natural, the hand
ends there. If not, each may be dealt a third card by fixed rules, the same
at every table: nobody chooses.

- The player's hand draws on 0 to 5 and stands on 6 or 7.
- If the player stood, the banker's hand does the same.
- If the player drew, the banker's hand draws on 0 to 2; on 3 unless the
  player's third card was an 8; on 4 if it was 2 to 7; on 5 if it was 4 to
  7; on 6 if it was 6 or 7; and stands on 7.

| Bet | Wins when | Pays |
|---|---|---|
| PLAYER | The player's hand wins | 1 to 1 |
| BANKER | The banker's hand wins | 1 to 1, but half on a winning 6 |
| TIE | The hands count the same | 8 to 1 |
| PLAYER PAIR | The player's first two cards are of one rank | 11 to 1 |
| BANKER PAIR | The banker's first two cards are of one rank | 11 to 1 |

This is the "no commission" game: a win on the banker is paid in full, with
nothing taken from it, except when the banker wins with a 6. On a tie, bets
on the two hands are handed back.

The chips are 10, 50, 100, 500 and 1000 credits, and one bet can hold up to
5,000. You can bet on several things at once. Chips on the table are paid
for when the hand is dealt, and no more can be put down than the bank holds.
After a hand the table is cleared, with the winning chips left on show until
you bet again. REBET puts the last bets back, and DEAL with nothing on the
table makes them again and deals.

With the mouse, click a bet to put a chip on it and right click to take it
off. The keyboard works too, with a marker on the bet you are at:

| Key | Action |
|---|---|
| Space | Deal |
| Left / Right | Move the marker from bet to bet |
| Enter | Put a chip where the marker is |
| Backspace | Take a chip off there |
| 1 to 5 | Choose a chip |
| X | Clear the table |
| B | Make the last bets again |
| C | Insert a coin |
| R | Reset your credits |
| S | Sound on or off |
| Esc | Back to the menu |

### The shoe

The game is dealt from eight decks, and the shoe is drawn in the corner of
the table as Blackjack's is. After each shuffle the first card is turned up
and as many cards burnt as it counts, ten for a ten or a picture: they are
dealt face down beside it, one at a time, before the first hand.

The cut card stands 14 cards from the end of the shoe. When it comes out it
is dealt onto the table as a plain yellow card, the hand is finished, and
one hand more is dealt. The shoe is then shuffled, which is shown as it is
in Blackjack. The row of last hands starts afresh with each shoe.

## Blackjack

Choose a bet, and a Perfect Pairs bet if you want one, then press DEAL. You
are dealt two cards and the dealer one. Get closer to 21 than the dealer
without going past it: pictures count 10 and an ace 1 or 11.

| Buttons | Choices |
|---|---|
| Bet | 10, 20, 50, 100 or 500 credits |
| Perfect Pairs bet | Off, or 5, 10, 25 or 50 credits |
| HIT | Take another card |
| STAND | Play the hand as it is |
| DOUBLE | Double the bet for one more card |
| SPLIT | Make two hands of two cards that count the same |
| HINT | Mark the best play |

The keyboard works too:

| Key | Action |
|---|---|
| Space or Enter | Deal |
| 1, 2, 3, 4 | Hit, stand, double, split |
| H | Hint: mark the best play |
| Up / Down | Change the bet |
| Left / Right | Change the Perfect Pairs bet |
| C | Insert a coin |
| R | Reset your credits |
| S | Sound on or off |
| Esc | Back to the menu |

### Rules

| Rule | How it is played here |
|---|---|
| Blackjack | An ace and a ten or a picture in your first two cards. It pays 3 to 2 |
| Dealer | Draws to 17 and stands on any 17, soft or hard |
| Double | On a hard 9, 10 or 11 in your first two cards only. A hand with an ace is soft: an ace and an eight is a soft 19, not a 9 |
| Split | Once, on any two cards that count the same, so a king and a queen can be split. Each hand can be doubled |
| Split aces | One card each. A ten on one makes 21, which pays even money: it is not a blackjack |
| Dealer's blackjack | Takes only the bet you first made. Anything added by doubling or splitting is returned, even on a hand that went bust |
| Push | The same total as the dealer returns your bet |

There is no insurance and no surrender.

The dealer has no hidden card. A card lies face down beside the dealer's
first for the look of the table, but no card has been drawn for it: the
dealer's second card is drawn after you have played. A dealer's blackjack
can therefore only turn up at the end of the hand, which is why it takes
your first bet and no more.

### The shoe

The game is dealt from six decks. Hands are dealt from the shoe one after
another, so a card that has been played stays out until the next shuffle.
At each shuffle a cut card is put in at random between two thirds and five
sixths of the way through, so between one deck and two are never dealt.
When the cut card comes out the hand being played is finished, and the shoe
is shuffled before the next.

The shoe is drawn at the end of the table, emptying as the cards are dealt,
with the cut card standing in it. It shows how far through the shoe is, not
a count.

The cut card is a plain yellow card. When it comes out it is dealt onto the
table beside the shoe, as a dealer sets it aside, and lies there while the
hand is finished from the cards behind it. If it comes out among the
dealer's cards it takes its turn as they are shown.

Pressing DEAL after that shows the shoe being shuffled before the hand is
dealt: the cards are riffled together a few times, and the shoe fills up
again. Space, Enter or a click cuts the shuffle short. It is for show: the
shoe is shuffled either way.

### Perfect Pairs

The side bet is on your first two cards being a pair, and is paid as soon
as they are dealt, whatever happens to the hand afterwards.

| Pair | Example | Pays |
|---|---|---|
| Perfect: the same card twice | Two kings of hearts | 25 to 1 |
| Coloured: the same rank and colour | King of hearts, king of diamonds | 12 to 1 |
| Mixed: the same rank, one red and one black | King of hearts, king of spades | 6 to 1 |

A pair here is two cards of the same rank. A king and a queen can be split,
but they are not a pair.

### Hints

HINT marks the best play and says what the hand is then worth on average.
It is worked out from the cards actually left in the shoe, so the advice
moves as the shoe runs down: it is the play a perfect card counter would
make.

It is exact for hit, stand and double. Splitting cannot be weighed up
exactly in any reasonable time, because the two hands share a shoe and a
dealer and the best play of one depends on every way the other could go. A
split is weighed up from the cards on the table, as if the other hand were
to draw no more. That is exact for the second of the two hands once the
first is finished, and close for the rest.

## Roulette

Choose a chip, put chips on the table and press SPIN. The ball comes to
rest on one of 37 numbers: 1 to 36 and a single zero.

With the mouse, click where the chip is to go and right click to take it
off again. A click in the middle of a number bets on the number; a click
near a line or a corner bets on the line or the corner.

| Bet | Where the chip goes | Pays |
|---|---|---|
| Straight up | On a number | 35 to 1 |
| Split | On the line between two numbers | 17 to 1 |
| Street | At the foot of a row of three | 11 to 1 |
| Corner | Where four numbers meet | 8 to 1 |
| Six line | At the foot of the line between two rows | 5 to 1 |
| Column, dozen | In its box | 2 to 1 |
| Red, black, odd, even, 1 to 18, 19 to 36 | In its box | 1 to 1 |

The zero splits with each of 1, 2 and 3, makes a street with 1 and 2 or
with 2 and 3, and a corner with all three. It is not red or black, odd or
even, high or low, and is in no column or dozen: all of those lose on it.

The chips are 10, 50, 100, 500 and 1000 credits, and one bet can hold up to
5,000. Chips on the table are paid for when the wheel is spun, and no more
can be put down than the bank holds. After a spin the table is cleared, with
the winning chips left on show until you bet again. REBET puts the last bets
back, and SPIN with nothing on the table makes them again and spins.

The keyboard works too. A marker shows the bet you are on, lights up the
numbers it covers, and its name and what it pays are shown under the table.

| Key | Action |
|---|---|
| Space | Spin |
| Arrows | Move the marker; hold one down to keep it moving |
| Enter | Put a chip where the marker is |
| Backspace | Take a chip off there |
| 1 to 5 | Choose a chip |
| X | Clear the table |
| B | Make the last bets again |
| C | Insert a coin |
| R | Reset your credits |
| S | Sound on or off |
| Esc | Back to the menu |

### The wheel

The wheel turns one way while the ball runs round the rim the other way.
The ball slows, drops, bounces across the pockets and comes to rest in one,
then rides round with the wheel as it stops. Space, Enter or a click cuts
the spin short. It is for show: the number is chosen the moment SPIN is
pressed. The numbers lie round the wheel as they do on a real one.

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
go test -v ./games/baccarat/table/
go test -v ./games/blackjack/table/
go test -v ./games/roulette/wheel/
go test -v ./games/slot/machine/
go test -v ./games/videopoker/poker/
```

Every card and every spin is drawn from a cryptographic random number
generator seeded by the operating system, and seeded afresh every hundred
hands or spins. No spin of the Slot Machine or the Roulette wheel and no hand
of Video Poker depends on the ones before it. Blackjack and Baccarat are
the exceptions by design: their hands come from one shoe until it is
shuffled.

### Baccarat

There is nothing to play well or badly, so what a bet pays back depends
only on the cards. From a full shoe of eight decks:

| Bet | How often it wins | Pays back |
|---|---|---|
| PLAYER | 44.62% of hands | 98.76% |
| BANKER | 45.86% of hands, 5.39% of them with a 6 | 98.54% |
| TIE | 9.52% of hands | 85.64% |
| Either PAIR | 31 times in 415 | 89.64% |

They are worked out by following every way a hand can come out of the shoe,
and move a little from hand to hand as the shoe runs down.

### Blackjack

From a freshly shuffled shoe, played perfectly without splitting, the game
pays back 98.94% of what is bet. Splitting when it is worth it pays back a
little more; that part is not worked out exactly, for the reason given
under Hints.

That figure is for the first hand after a shuffle. As the shoe runs down,
what a hand pays back moves with the cards left in it, and the return over
a whole shoe dealt down to the cut card is not worked out.

Perfect Pairs pays back 93.89% of what is bet on it from a freshly shuffled
shoe, and that too moves as the shoe runs down:

| Pair | How often | Share of what is bet |
|---|---|---|
| Perfect | 5 times in 311 | 41.80% |
| Coloured | 6 times in 311 | 25.08% |
| Mixed | 12 times in 311 | 27.01% |

A pair of some kind comes up about once in 13.5 hands.

### Roulette

Each of the 37 pockets is as likely as any other, and every bet pays as if
there were 36. So every bet on the table pays back the same, 36 parts in
37, which is 97.30% of what is bet, however the game is played.

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
- `ui/` has the palette, text, shapes, buttons, menus, card drawing and
  sound that every game uses.
- `shoe/` holds the shoe that Blackjack and Baccarat are dealt from.
- `shuffle/` shows a shoe being shuffled, for any game that has one.
- `games/baccarat/` has Baccarat's screens, with its rules in
  `games/baccarat/table/`.
- `games/blackjack/` has Blackjack's screens, with its rules in
  `games/blackjack/table/`.
- `games/roulette/` has Roulette's screens, with its rules in
  `games/roulette/wheel/`.
- `games/slot/` has the Slot Machine's screens, with its rules in
  `games/slot/machine/`.
- `games/videopoker/` has Video Poker's screens, with its rules in
  `games/videopoker/poker/`.

The rules packages, `bank/`, `cards/` and `shoe/` have no graphics and are covered
by tests. The art is drawn from simple shapes and the sound is generated,
so there are no asset files.

## Licence

Released under the MIT licence. See `LICENSE`.
