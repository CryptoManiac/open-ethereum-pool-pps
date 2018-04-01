# PoT share reward for various a and X values.

## PoT description
PoT (Pay on Target) is a configurable variant of PPS system which allows you to customize miner's variance. By playing a and X factors you can literally turn it into plain PPS, solo mining or something moderate between PPS and solo mining.

## Examples

### Common variables

shareDiff = 4294967296

netDiff = 3.211 P

fee = 1.5%

### Moderate variance configuration
a=0.800000 X=5.000000
```
5G share reward: 0.00000092873202164 ETH
20 G share reward: 0.00000281538902210 ETH
80 G share reward: 0.00000853466356393 ETH
320 G share reward: 0.00002587226190688 ETH
1280 G share reward: 0.00007843003197071 ETH
5120 G share reward: 0.00023775539753990 ETH
NetDiff/4 share reward: 0.01356371537117023 ETH
NetDiff/3 share reward: 0.01707377905157401 ETH
NetDiff/2 share reward: 0.02361580011352108 ETH
Maximum share reward: 0.14900562527402308 ETH
Block solving share reward: 0.04111749618302889 ETH
```
### Big variance configuration
a=0.900000 X=4.000000
```
5G share reward: 0.00000056835189606 ETH
20 G share reward: 0.00000197911625307 ETH
80 G share reward: 0.00000689168307577 ETH
320 G share reward: 0.00002399823433467 ETH
1280 G share reward: 0.00008356670567262 ETH
5120 G share reward: 0.00029099617078442 ETH
NetDiff/4 share reward: 0.02752115093104003 ETH
NetDiff/3 share reward: 0.03565426228924745 ETH
NetDiff/2 share reward: 0.05135628356744854 ETH
Maximum share reward: 0.33371411516488336 ETH
Block solving share reward: 0.09583421378229816 ETH
```
### Solo-like configuration
a=20.000000 X=1.000000
```
5G share reward: 0.00000000000000000 ETH
20 G share reward: 0.00000000000000000 ETH
80 G share reward: 0.00000000000000000 ETH
320 G share reward: 0.00000000000000000 ETH
1280 G share reward: 0.00000000000000000 ETH
5120 G share reward: 0.00000000000000000 ETH
NetDiff/4 share reward: 0.00000000000255318 ETH
NetDiff/3 share reward: 0.00000000080511144 ETH
NetDiff/2 share reward: 0.00000267720222473 ETH
Maximum share reward: 2.80724999999999936 ETH
Block solving share reward: 2.80724999999999936 ETH
```
## References

https://bitcointalk.org/index.php?topic=131376.0 - Original thread on bitcointalk.org

https://play.golang.org/p/bEoEazMwDqn - Script used to calculate rewards in this note
