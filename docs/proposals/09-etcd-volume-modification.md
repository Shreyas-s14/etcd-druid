---
title: Modifying etcd member volumes (capacity and storage class)
dep-number: 09
creation-date: 
status: 
authors:
reviewers:
- "@etcd-druid-maintainers"
---

# DEP-09: Modifying etcd member volumes managed by `etcd-druid`

## Summary

- `etcd-druid` today has no path to change an existing etcd member's volume — neither its capacity nor its `StorageClass`.
- The blocker is Kubernetes: a StatefulSet's `.spec.volumeClaimTemplates` is immutable, so the desired volume spec cannot be rolled to members through the normal reconcile.
- This DEP records three candidate methods for effecting such a change — **Method 1 (cascade-orphan rebuild)**, **Method 2 (bootstrap + scale-in parallel cluster)**, and **Method 3 (hybrid: companion cluster + in-place rebuild)** — and their trade-offs, so a direction can be chosen.

## Terminology

- **Volume modification** — any change to an etcd member's volume: capacity increase (scale-up), capacity decrease (scale-down), or `StorageClass` migration.
- **`volumeClaimTemplates`** — the StatefulSet field that provisions per-member PVCs; immutable after creation.
- **`bootstrapWithExistingCluster`** — mechanism by which a new `Etcd` joins an existing cluster instead of forming its own. See [Bootstrap with an Existing etcd Cluster](../concepts/bootstrap-with-existing-cluster.md).
- **Scale-in** — safe removal of members via the `Etcd` API. See [DEP-08](./08-scale-in.md).
- **Companion cluster** — a temporary `Etcd` resource whose members join the target cluster only to hold quorum during an operation, then are removed.

## Motivation

- Operators need to replace unencrypted volumes with encrypted ones (encryption default changed >1y ago; adopters still sit on unencrypted volumes).
- Operators need to replace over-/under-sized volumes with correctly sized ones (e.g. on AWS, gp3 removes the need to over-provision for IOPS).
- These require changing volume capacity or `StorageClass` on a live etcd cluster — ideally without downtime for HA clusters.

### Why this is not doable directly through the StatefulSet

- The StatefulSet controller does not watch or operate on PVCs.
- Its controller-revision hash — which drives pod rolling — is computed from the pod template only, **not** from `volumeClaimTemplates`; changing the volume spec does not roll pods.
- A PVC can only be **expanded**, and only if the `StorageClass` has `allowVolumeExpansion: true`.
- **Shrinking is not possible in a live cluster**: PVC API validation forbids `spec.Capacity` below `spec.Requests`, and shrinking would require unmounting and offline resizing — no safe path.
- `StorageClass` is consumed only at volume-creation time and is **immutable** on an existing PVC — there is no in-place path at all.
- Therefore the only general mechanism is: **create a new volume and let etcd catch up via the learner mechanism.**

### Operations in scope

- **Scale-Up** — grow each member's PVC capacity. *Has an in-place shortcut* (`allowVolumeExpansion`), so a rebuild is avoidable here.
- **Scale-Down** — shrink each member's PVC capacity. *No in-place path* — requires new volumes.
- **StorageClass migration** — move each member's PVC to a different `StorageClass`. *No in-place path* — requires new volumes.

### Goals

- Provide a mechanism to change etcd member volume capacity and `StorageClass`.
- Preserve etcd data and cluster quorum throughout.
- For HA clusters, avoid downtime.
- Guard the inherently-destructive volume deletion behind explicit operator confirmation.

### Non-Goals

- Automatic capacity scale-up where `allowVolumeExpansion` suffices — that should short-circuit to in-place PVC expansion and not trigger a rebuild at all.
- Cross-`StorageClass` data conversion beyond what etcd's learner resync already provides.

## Proposal

Three methods are recorded. All assume an HA (multi-node) cluster unless noted; deleting a PVC destroys that member's data, so quorum must cover the wipe.

### Method 1: Cascade-orphan rebuild

Steps:

- Delete the StatefulSet with `--cascade=orphan` so running etcd member pods survive.
- Change the `Etcd` spec (new `storageCapacity` / `storageClass`) and let the reconciler recreate the StatefulSet with the updated `volumeClaimTemplates`; it re-adopts the orphaned pods by label selector without restarting them.
- Pods continue to use their existing (old-spec) PVCs.
- Per member, one at a time: delete the PVC (marks it `Terminating` via the `pvc-protection` finalizer; the object survives while the pod holds it), then delete the pod. The StatefulSet controller provisions a fresh pod with a new PVC at the new spec; the member resyncs.
- Repeat for all members in a quorum-aware manner (borrowing the health-gated, follower-before-leader ordering from [DEP-07](./07-quorum-aware-pod-updates.md)) so quorum is held throughout.

Cons:

- Transient reduced redundancy during each rebuild — there is a window with no replica manager for the member while it resyncs, extendable by reconcile errors.
- Per-step quorum checks (as in DEP-07) can slow the overall process.
- Slow: each member fully repopulates its data dir (leader-streamed snapshot to a fresh learner), repeated `n` times for `n` members.
- Single-node clusters need a separate path (snapshot, then change) since there is no peer to hold quorum during the wipe.

### Method 2: BootstrapWithExistingCluster + Scale-In (parallel cluster)

*Determined not viable — recorded for completeness.*

Steps:

- Create a new `Etcd` (`etcd-new`, e.g. 3 members) with `bootstrapWithExistingCluster` pointed at the old cluster's endpoints, and with the new volume spec.
- Wait for the new members to join via the learner mechanism and sync.
- Scale-in the old members by clearing the bootstrap field ([DEP-08](./08-scale-in.md) `BootstrapMembersRemoval`), removing old members one per reconcile.
- Once all old members are removed, delete the old `Etcd` resource — deleting its StatefulSet and old PVCs/PVs.

Cons (why it is not applicable):

- The surviving cluster is permanently named `etcd-new`; `metadata.name` is immutable. Gardener hardcodes `etcd-main` everywhere, and the metric `role` label equals the `Etcd` resource name (ServiceMonitor relabels `part-of` = `etcd.Name` → `role`). Everything filtering `role="etcd-main"` goes blind.
- The control-plane mutator webhook switches on the name and silently skips a renamed `Etcd`; health checks expecting `ETCDMain` fail; backup-bucket prefix and NetworkPolicies are name-derived.
- Unnecessary leader change(s), and PVCs may remain undeleted due to missed cycles.

### Method 3: Hybrid — companion cluster + in-place rebuild of the old STS

Combines Method 2's redundancy with Method 1's in-place resize, so the surviving cluster keeps its original name.

Steps:

- Create `etcd-new` (2–3 members) with `bootstrapWithExistingCluster` pointing at the old cluster → new members join as learners, sync, promote to voters → joint cluster (e.g. old 3 + new 2 = 5, quorum 3).
- The companion members are managed by their own StatefulSet, so they hold quorum and keep serving even while old members are taken down.
- Apply **Method 1 on the OLD StatefulSet**: orphan-cascade-delete → recreate with new `volumeClaimTemplates` → re-adopt old pods → roll old members one-by-one (delete PVC + pod → fresh PVC at new storage → resync). Companions cushion quorum throughout.
- Once all old members are rebuilt on new storage, remove the companions (scale-in) and delete the `etcd-new` resources.

Cons — resolved from Methods 1 & 2:

- Method 1's transient quorum-loss risk during rebuild → cushioned by the companions.
- Method 1's single-node gap → the old cluster is HA (joint) during the resize; no restore-from-backup path needed.
- Method 2's `etcd-main` rename problem → survivors are the old resized members; the name stays `etcd-main` (metrics/webhook/health intact).

Cons — new / remaining:

- **Companion teardown has no supported DEP-08 mechanism.** Removing the companions is an `N → 0` scale-in, an explicit DEP-08 Non-Goal; `BootstrapMembersRemoval` targets the old members, not the companions; a plain resource-delete does not guarantee a clean `MemberRemove` per companion → risk of orphaned dead voters in the surviving cluster's membership. This teardown path must be built from scratch.
- Needs the **union of all unbuilt dependencies**: DEP-07 (rolling/quorum gate) + bootstrap (built) + DEP-08 scale-in + anti-rejoin guard — spanning three repos — plus the new teardown.
- **Operates on a joined cluster, which DEP-08 explicitly warns against** ([DEP-08 CAUTION](./08-scale-in.md)): the old cluster is the *source*, and Method-1 rebuild churns membership on the source side while two un-coordinated `Etcd` controllers reconcile one joint cluster (CEL is scoped per-resource; cross-resource quorum is not admission-coordinated).
- Method 1's mechanics remain fully present on the old STS (orphan window, fragile re-adoption, full per-member repopulation) — companions cushion quorum but remove none of it.
- Anti-rejoin guard interactions are under-specified when a Method-1 rebuild wipes and rejoins an old member inside a joint cluster.
- Extra churn: companions full-sync on join and are torn down afterwards — data movement for scaffolding only.
- Highest implementation complexity of the three — combines two state machines each designed to run alone.

## Trigger and safeguards (applies to all methods)

- The destructive PVC deletion must be gated behind an explicit confirmation annotation (e.g. `druid.gardener.cloud/pvc-deletion-confirmation`), and druid must take a full snapshot before any member's volume is wiped.
- **StorageClass migration must be triggerable even when the `StorageClass` *name* is unchanged.** A `StorageClass` is consumed only at volume-creation time, so its *contents* can be edited (e.g. in-tree provisioner → CSI) while the name stays the same; the PV froze the old definition. So the trigger cannot be a naive `storageClass` field diff — it needs an explicit force/confirm signal (the same confirmation annotation can serve both purposes).
- **Capacity scale-up should short-circuit to in-place expansion** where `allowVolumeExpansion: true`, bypassing all three rebuild methods; the methods above are justified by scale-down and StorageClass migration, where no in-place path exists.

## Open questions

- Can the trigger originate outside etcd-druid's scope (gardenlet, seed extension webhook, migration), and how is that reconciled with druid's own reconcile trigger?
- HA resource availability: rebuilding creates new PVCs; worker-pool / zone / node constraints may prevent scheduling (etcds often land on the same nodes), forcing rollback.
- How many companion members should Method 3 create, and how is that decided?
- Resource exhaustion is possible for Method 2 and Method 3 in the Gardener case.

## Alternatives

- The three methods above are alternatives to one another; Method 1 is the leading candidate for keeping identity and avoiding cross-resource coordination, Method 3 the leading candidate where transient quorum safety must be guaranteed for small clusters.
