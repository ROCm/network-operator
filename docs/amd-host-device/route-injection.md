# AMD Host Device CNI — Policy Route Injection via NAD

## Background

The `amd-host-device` CNI plugin moves a host NIC into a pod's network namespace using
the `host-device` sub-plugin, and synthesises a static IPAM config from the addresses
already configured on that NIC. When chained with the `sbr` (Source-Based Routing) CNI
plugin, SBR reads the IPAM result and installs policy routing rules so that traffic
sourced from the secondary NIC IP is steered back through the correct interface.

Previously, the only routing information propagated to SBR was an implicit default
gateway derived for `/31` point-to-point links. For aggregated spine-fabric topologies —
where a pod needs to reach a supernet prefix (e.g. `/19`) via its ToR peer — there was
no mechanism to inject an explicit route into the IPAM result without additional CNI
plugins or manual host configuration.

---

## Change Summary

Two new optional fields are introduced in the NetworkAttachmentDefinition (NAD) config
block for `amd-host-device`:

| Field                  | Type           | Description                                                                          |
|------------------------|----------------|--------------------------------------------------------------------------------------|
| `nexthopNetAddrOffset` | integer        | Offset added to the network address of the interface subnet to derive the nexthop IP |
| `routeDstPrefixLen`    | integer (0-32) | Prefix length applied to the interface IP to derive the route destination            |

When both fields are present, the plugin computes one route per IPv4 address on the host
interface and injects them into the static IPAM `routes` array. SBR then installs these
routes into the per-source policy routing table it manages.

If either field is absent, no routes are injected. Note that this is not identical to the
previous release for `/31` links: the implicit `/31` gateway injection has been removed
(see *Removed Behaviour* below), so a `/31` NAD without these fields no longer produces a
route.

---

## Route Derivation Algorithm

For each IPv4 address on the host interface:

**Destination:**

```text
dstIP  = interfaceIP & CIDRMask(routeDstPrefixLen, 32)
dstNet = dstIP / routeDstPrefixLen
```

**Nexthop — two cases:**

- **/31 point-to-point links:** the nexthop is always the XOR peer
  (`interfaceIP ^ 1` on the last octet). This is correct regardless of which end of
  the /31 the NIC holds, and `nexthopNetAddrOffset` is ignored for this case.

- **All other prefix lengths:** the nexthop is derived as:

  ```text
  networkAddr = interfaceIP & subnetMask
  nexthop     = networkAddr + nexthopNetAddrOffset
  ```

The resulting route entry added to the IPAM `routes` array:

```json
{ "dst": "<dstNet>", "gw": "<nexthop>" }
```

---

## Examples

### Non-/31 — spine supernet route

NIC: `10.1.2.130/25`, `nexthopNetAddrOffset=1`, `routeDstPrefixLen=19`

```text
networkAddr = 10.1.2.128
nexthop     = 10.1.2.128 + 1         = 10.1.2.129
dstIP       = 10.1.2.130 & /19 mask  = 10.1.0.0
route       → 10.1.0.0/19 via 10.1.2.129
```

SBR installs in the policy table:

```text
ip rule:  from 10.1.2.130 lookup table 100
table 100: 10.1.0.0/19 via 10.1.2.129
```

Traffic sourced from `10.1.2.130` destined for anything in `10.1.0.0/19` is
policy-routed via the ToR peer `10.1.2.129`.

---

### Non-/31 — explicit default route

NIC: `10.1.2.130/25`, `nexthopNetAddrOffset=1`, `routeDstPrefixLen=0`

```text
nexthop = 10.1.2.129
dstNet  = 0.0.0.0/0
route   → 0.0.0.0/0 via 10.1.2.129
```

---

### /31 — spine supernet route

NIC: `10.1.2.5/31`, `nexthopNetAddrOffset=0` (ignored), `routeDstPrefixLen=16`

```text
nexthop = 10.1.2.5 XOR 1             = 10.1.2.4
dstIP   = 10.1.2.5 & /16 mask        = 10.1.0.0
route   → 10.1.0.0/16 via 10.1.2.4
```

Works correctly regardless of whether the NIC holds the lower (`.4`) or upper (`.5`)
address of the /31 pair.

---

### /31 — explicit default route

`nexthopNetAddrOffset=0`, `routeDstPrefixLen=0` → `0.0.0.0/0 via 10.1.2.4`

---

## Removed Behaviour

The previous implicit `/31` gateway injection — where the XOR peer was placed in the
`gateway` field of the IPAM `addresses` entry causing SBR to install an implicit default
route — has been removed. The `routeDstPrefixLen=0` mechanism replaces it explicitly and
extends the same capability to non-/31 prefixes. This makes all routing behaviour opt-in
and fully visible in the NAD, with no implicit defaults.

---

## NAD Example

```json
{
  "cniVersion": "0.3.1",
  "name": "vf-amd-host-device-sbr-nad",
  "plugins": [
    {
      "type": "amd-host-device",
      "nexthopNetAddrOffset": 1,
      "routeDstPrefixLen": 19
    },
    { "type": "sbr" }
  ]
}
```

## Notes

- Both fields must be specified together; if either is absent no routes are injected.
- `routeDstPrefixLen` must be **strictly less than** the interface prefix length.
  Equal or longer values skip that address (logged); they would be a connected-subnet
  or host route via the peer, which is not useful. `0` is always a supernet and
  installs a default route (`0.0.0.0/0`), which is how you restore the old `/31` + SBR
  gateway.
- For non-`/31` addresses, `nexthopNetAddrOffset` must fall in
  `[0, 2^(32 − interface prefix) − 1]` so the nexthop stays inside the interface
  subnet (typical ToR peer is `1`). Out of range skips that address (logged).
  On `/31` the offset is ignored and the XOR peer is used.
- Only IPv4 addresses are processed; IPv6 addresses on the interface are unaffected.
- Multiple IPv4 addresses on the same interface each produce an independent route entry.
