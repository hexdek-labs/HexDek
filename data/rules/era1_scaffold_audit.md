# Era 1 (1993-2014) Scaffold-Gap Audit

- Total cards in dataset: **31963**

- Era distribution: era1=26932, era2=537, era3=798, era4=3696

- Era 1 cards: **26932**

- Era 1 Condition nodes: **2499** (bucketed 2470, unbucketed 29, 1.2% gap)

- Era 1 Trigger nodes: **11548** (bucketed 11548, unbucketed 0, 0.0% gap)


## Top unbucketed condition Kinds

- `conditional` × 20
- `if` × 9

## Top unbucketed raw-text fragments (kind in raw/intervening_if/as_long_as)

- × 1: `as long as this creature has a counter on it, it can attack as though it didn't `  _(e.g. Demon Wall)_
- × 1: `if you would flip a coin, you may instead roll a d20. 1−10 is tails and 11−20 is`  _(e.g. Pokey, the Scallywagg)_
- × 1: `it isn't being declared as an attacker`  _(e.g. Verity Circle)_
- × 1: `at least one other wall creature is blocking that creature and no non-wall creat`  _(e.g. Wall of Caltrops)_
- × 1: `if that library contains exactly the chosen number of cards with the chosen name`  _(e.g. Mindblaze)_
- × 1: `if that creature is attacking, x is the number of snow lands defending player co`  _(e.g. Snowblind)_
- × 1: `if an instant card or a card with flash is exiled this way, they lose 1 life. ot`  _(e.g. Check for Traps)_
- × 1: `if that card is returned to its owner's hand this way, you may pay {u}{u}{u}. if`  _(e.g. Puppet Master)_
- × 1: `if any of those cards shares a card type with that spell, copy that spell, you m`  _(e.g. Gandalf, Westward Voyager)_
- × 1: `if they do, exile that card. otherwise, put it into your hand`  _(e.g. Protection Racket)_
- × 1: `the spell's mana value is less than or equal to the amount of life you gained th`  _(e.g. Bre of Clan Stoutarm)_
- × 1: `as long as this card is face up during the draft, you can't look at booster pack`  _(e.g. Archdemon of Paliano)_
- × 1: `if a player does, this creature deals damage to that player equal to that card's`  _(e.g. Sin Prodder)_
- × 1: `target attacking creature token is the chosen type`  _(e.g. A Killer Among Us)_
- × 1: `if you sacrifice a snow forest this way, this creature gains trample until end o`  _(e.g. Gargantuan Gorilla)_
- × 1: `if they do, you may discard a card. if you do, draw a card`  _(e.g. Tibalt, Wicked Tormentor)_
- × 1: `if you win, target player discards two cards. otherwise, that player discards a `  _(e.g. Pulling Teeth)_
- × 1: `you've done all four this turn`  _(e.g. Avatar Aang // Aang, Master of Elements)_
- × 1: `if they do, they put a rope counter on a creature they control. otherwise, exile`  _(e.g. Fraying Line)_
- × 1: `it isn't a mana ability`  _(e.g. Battlemage's Bracers)_
- × 1: `as long as ~ is on the stack, spells that target it cost {2} more to cast`  _(e.g. Kaervek's Torch)_
- × 1: `as long as there are three or more cards exiled with this creature, it gets +3/+`  _(e.g. Veteran Survivor)_
- × 1: `if they match, sacrifice this artifact and put all cards exiled with it into the`  _(e.g. Expert-Level Safe)_
- × 1: `if excess damage was dealt this way, note that excess damage, then you get a one`  _(e.g. Molten Impact)_
- × 1: `you guessed correctly for a card named ~`  _(e.g. Spire Phantasm)_
- × 1: `there were three or more card types among the sacrificed permanents`  _(e.g. Baba Lysaga, Night Witch)_
- × 1: `if no counters were removed this way, put a flood counter on this enchantment an`  _(e.g. Bounty of the Luxa)_
- × 1: `if they can't, they lose 2 life and you draw a card. then repeat this process fo`  _(e.g. Invoke Despair)_
- × 1: `rasputin started the turn untapped`  _(e.g. Rasputin Dreamweaver)_

## Bucketed condition Kinds (sanity)

- `if` × 705
- `paid_optional_cost` × 561
- `conditional` × 401
- `for_each` × 193
- `did_prior_action` × 137
- `etb_tapped_unless` × 97
- `threshold` × 64
- `delirium` × 40
- `no_spells_cast_last_turn` × 32
- `two_plus_spells_cast_last_turn` × 32
- `domain` × 20
- `etb_if` × 20
- `hellbent` × 14
- `you_attacked_this_turn` × 13
- `ferocious` × 13
- `raid` × 13
- `spell_mastery` × 12
- `repeat_n` × 12
- `morbid` × 9
- `you_control` × 9

## Top unbucketed trigger events

_(none — every Era 1 trigger event maps to a scaffold slug)_

## Top trigger events (bucketed + unbucketed)

- `etb` × 3252
- `phase` × 1628
- `attack` × 850
- `die` × 529
- `combat_damage_player` × 447
- `cast_filtered` × 414
- `etb_as` × 225
- `cast_any` × 217
- `tribe_you_control_etb` × 206
- `when_you_do` × 144
- `beginning_of_ordinal_step` × 110
- `another_typed_enters` × 104
- `any_player_cast` × 98
- `etb_or_another` × 96
- `enter_or_attack` × 89
- `becomes_blocked` × 86
- `turned_face_up` × 83
- `cast_color_spell` × 78
- `cast_spell` × 77
- `deals_damage` × 77
- `becomes_tapped` × 76
- `you_whenever` × 75
- `you_attack` × 74
- `dealt_damage` × 71
- `creature_dies` × 70
- `gain_life` × 69
- `becomes_target` × 65
- `opp_cast` × 64
- `sacrifice_filtered` × 58
- `block_or_becomes_blocked` × 53
- `attack_or_block` × 50
- `cycle` × 49
- `self_and_another` × 46
- `group_combat_damage_player` × 43
- `nontoken_ally_event` × 42
- `one_or_more_typed_event` × 42
- `attack_alone` × 40
- `block` × 37
- `group_attack` × 36
- `self_deals_damage_player` × 34