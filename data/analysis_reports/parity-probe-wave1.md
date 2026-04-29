# Go ↔ Python Parity Report

_Generated: 2026-04-16T16:07:14-07:00_

| field | value |
|---|---|
| games | 10 |
| n_seats | 4 |
| base_seed | 42 |
| python_available | true |
| outcome_match | 1/10 (10.0%) |
| event_stream_match | 0/10 (0.0%) |

## Deck Lineup

0. `data/decks/cage_match/nazgul_tribal_control_b3_lord_of_the_nazgul.txt`
1. `data/decks/cage_match/sin_landreanimator_b3_spiras_punishment.txt`
2. `data/decks/cage_match/tokens_widetall_b3_maja.txt`
3. `data/decks/cage_match/werewolf_daynight_flip_b3_ulrich.txt`

## Divergence Categories

| category | count |
|---|---|
| event_count | 173 |
| event_missing_go | 156 |
| event_missing_py | 70 |
| outcome | 9 |
| turn_count | 10 |

## Divergences (first 50)

- game=0 cat=outcome — go_winner=1 go_end=last_seat_standing py_winner=2 py_end=seat 2 is the last one standing
- game=0 cat=event_missing_go — kind="add_mana" go=0 py=215
- game=0 cat=event_count — kind="attackers" go=19 py=37
- game=0 cat=event_count — kind="blockers" go=19 py=37
- game=0 cat=event_count — kind="cast" go=31 py=52
- game=0 cat=event_count — kind="commander_cast_from_command_zone" go=3 py=5
- game=0 cat=event_missing_go — kind="commander_damage_accum" go=0 py=8
- game=0 cat=event_missing_go — kind="conditional_evaluated" go=0 py=1
- game=0 cat=event_count — kind="damage" go=51 py=104
- game=0 cat=event_missing_go — kind="damage_wears_off" go=0 py=17
- game=0 cat=event_missing_go — kind="destroy" go=0 py=11
- game=0 cat=event_count — kind="discard" go=1 py=2
- game=0 cat=event_count — kind="draw" go=41 py=58
- game=0 cat=event_count — kind="enter_battlefield" go=18 py=41
- game=0 cat=event_missing_py — kind="game_end" go=1 py=0
- game=0 cat=event_missing_go — kind="life_change" go=0 py=77
- game=0 cat=event_missing_go — kind="mill" go=0 py=1
- game=0 cat=event_count — kind="pay_mana" go=34 py=57
- game=0 cat=event_missing_go — kind="per_card_unhandled" go=0 py=2
- game=0 cat=event_missing_py — kind="phase_step" go=84 py=0
- game=0 cat=event_missing_go — kind="phase_step_change" go=0 py=630
- game=0 cat=event_count — kind="play_land" go=27 py=31
- game=0 cat=event_missing_go — kind="pool_drain" go=0 py=24
- game=0 cat=event_count — kind="priority_pass" go=175 py=161
- game=0 cat=event_missing_go — kind="recurse" go=0 py=2
- game=0 cat=event_missing_go — kind="replacement_registered" go=0 py=1
- game=0 cat=event_count — kind="resolve" go=16 py=17
- game=0 cat=event_missing_py — kind="sba_704_5f" go=1 py=0
- game=0 cat=event_count — kind="sba_704_5g" go=6 py=9
- game=0 cat=event_count — kind="sba_704_6d" go=1 py=2
- game=0 cat=event_count — kind="sba_cycle_complete" go=8 py=9
- game=0 cat=event_missing_go — kind="stack_counter" go=0 py=1
- game=0 cat=event_count — kind="stack_push" go=67 py=57
- game=0 cat=event_count — kind="stack_resolve" go=67 py=57
- game=0 cat=event_missing_go — kind="state" go=0 py=111
- game=0 cat=event_count — kind="turn_start" go=42 py=56
- game=0 cat=event_missing_go — kind="tutor" go=0 py=3
- game=0 cat=event_missing_py — kind="unknown_effect" go=35 py=0
- game=0 cat=event_missing_go — kind="untap_done" go=0 py=260
- game=0 cat=turn_count — go_turns=42 py_turns=56
- game=1 cat=outcome — go_winner=3 go_end=last_seat_standing py_winner=2 py_end=seat 2 is the last one standing
- game=1 cat=event_missing_go — kind="add_mana" go=0 py=214
- game=1 cat=event_count — kind="attackers" go=17 py=26
- game=1 cat=event_count — kind="blockers" go=17 py=26
- game=1 cat=event_missing_py — kind="buff" go=1 py=0
- game=1 cat=event_count — kind="cast" go=32 py=48
- game=1 cat=event_count — kind="commander_cast_from_command_zone" go=2 py=5
- game=1 cat=event_missing_go — kind="commander_damage_accum" go=0 py=4
- game=1 cat=event_missing_go — kind="conditional_evaluated" go=0 py=1
- game=1 cat=event_missing_py — kind="create_token" go=1 py=0
