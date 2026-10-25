# Changelog

All notable changes to Video Casino are listed here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0](https://github.com/comprofix/videocasino/compare/v1.0.0...v1.1.0) - 2026-10-25

### Changed

- Everything is repainted in the Resurrect 64 palette: the main menu, the
  menus and confirm box, the cards and chips, and each game's felt and
  border.
- The slot has dark reels in a lavender frame, and every payline is easy
  to tell from the frame. The jackpot bars keep their colours.
- The program icon is repainted to match.

## [1.0.0](https://github.com/comprofix/videocasino/releases/tag/v1.0.0) - 2026-10-24

### Added

- Nine casino games under one main menu: Baccarat, Blackjack, Caribbean
  Stud, Pai Gow Poker, Roulette, Slot Machine, Three Card Poker, Ultimate
  Texas Hold'em and Video Poker, each with its own help and about screens.
- One bank of pretend credits shared by every game, starting at $1,000,
  kept in `save.json` beside the program with each game's own state.
- INSERT COIN adds $1,000 while the balance stays at or under $5,000.
- RESET sets the balance back to $1,000, after asking first.
- Card games shuffle on screen, from a shoe dealt down to a cut card or a
  single deck shuffled before every hand.
- Hints and returns worked out exactly from the rules, with Pai Gow
  Poker's return the one estimate.
- A slot machine program icon on the window and the taskbar.
- Release builds for Windows and Linux.
