------------------------------ MODULE PODLBFT ------------------------------
EXTENDS Naturals, FiniteSets, TLC

CONSTANT Validators, Byzantine, MaxFaults

Blocks == {"A", "B"}
Honest == Validators \ Byzantine
Quorum(S) == Cardinality(S) * 3 > Cardinality(Validators) * 2

VARIABLES prevotes, precommits, locks, decisions
vars == <<prevotes, precommits, locks, decisions>>

Init == /\ prevotes = [b \in Blocks |-> {}]
        /\ precommits = [b \in Blocks |-> {}]
        /\ locks = [v \in Honest |-> "nil"]
        /\ decisions = {}

CanHonestVote(v, b, votes) ==
    v \in Honest => \A other \in Blocks \ {b}: v \notin votes[other]

Prevote(v, b) ==
    /\ v \notin prevotes[b]
    /\ CanHonestVote(v, b, prevotes)
    /\ prevotes' = [prevotes EXCEPT ![b] = @ \cup {v}]
    /\ UNCHANGED <<precommits, locks, decisions>>

Precommit(v, b) ==
    /\ Quorum(prevotes[b])
    /\ v \notin precommits[b]
    /\ CanHonestVote(v, b, precommits)
    /\ (v \in Honest => (locks[v] = "nil" \/ locks[v] = b))
    /\ precommits' = [precommits EXCEPT ![b] = @ \cup {v}]
    /\ locks' = IF v \in Honest THEN [locks EXCEPT ![v] = b] ELSE locks
    /\ UNCHANGED <<prevotes, decisions>>

Decide(b) ==
    /\ Quorum(precommits[b])
    /\ decisions' = decisions \cup {b}
    /\ UNCHANGED <<prevotes, precommits, locks>>

Next == (\E v \in Validators, b \in Blocks: Prevote(v,b))
     \/ (\E v \in Validators, b \in Blocks: Precommit(v,b))
     \/ (\E b \in Blocks: Decide(b))

TypeOK == /\ prevotes \in [Blocks -> SUBSET Validators]
          /\ precommits \in [Blocks -> SUBSET Validators]
          /\ locks \in [Honest -> (Blocks \cup {"nil"})]
          /\ decisions \subseteq Blocks
FaultBound == Cardinality(Byzantine) <= MaxFaults
HonestPrevoteNonEquivocation == \A v \in Honest: ~(v \in prevotes["A"] /\ v \in prevotes["B"])
HonestPrecommitNonEquivocation == \A v \in Honest: ~(v \in precommits["A"] /\ v \in precommits["B"])
Agreement == Cardinality(decisions) <= 1

Spec == Init /\ [][Next]_vars
=============================================================================
