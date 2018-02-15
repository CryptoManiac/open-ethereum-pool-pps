# Rinning a PPS mining pool for beginners

First, I'll try to formulate a set of simple principles.

1. Keep in mind that running PPS pool is RISKY. If you're not ready for that, please consider using rinsk-free PPLNS system.
2. Don't overestimate your ability to balance the incomings and payments. Mining is basically a roulette and outcomes of wrong decisions may be catastrophic.
3. Keep your service as simple as possible. Please avoid implementation of unnecessary functionality. Your life is dependent on that, literally.

# Balancing the risks

In PPS system there is no luck for users, entire variance has to be balanced by the pool operator. This means that sometimes you will have fantastic income, but in other times you will have to pay more than you're earning.

There are three factors in the PPS system, which are dependent on each other: risk of bankruptcy, mining fee and amount of reserves.

## Risk of bankruptcy

First, estimate your accepted risk of bankruptcy. For example, you can be ready to operate with 1/10 or 1/1000 chances of bankruptcy. It's obvious that smaller chance of bankruptcy is better for you.

## Mining fee

Examine the market in order to discover acceptable fee ratio. FOr example, if there are no PPS pools with fee less than 2% then you can start with any value lower than or equal to 2%. You know, that's how free market works.

## Estimate amount of reserves

Amount of required reserves is directly dependent on the block reward (B), mining fee (F) and risk of bankruptcy (P) and may be calculated using the following equation:

```R = B * ln(1/P) / (2 * F)```

Just for example, let's pretend that you wish to mine ether with accepted risk of bankruptcy 1/1000 and your mining fee is 2% (i.e. 0.02). Giving these values into account, your required amount of reserve funds would be:

```3 * ln(1/0.001) / (2 * 0.02)```

So you need to have 518.08 ETH locked in some offline wallet, just in case.

# Veryfing that you're ready

You have estimated your risks and the basic parameters are chosen. It's always better to re-check your risk of bankruptcy using the following formula:

P = EXP(-2 * F * R / B)

With R=300 ETH and your F=0.02 your probability of eventual bankruptcy will be EXP(-2 * 0.02 * 300 / 3) ~= 0.0183. If you're OK with that, then proceed.

# Staying simple

Keep in mind that implementation of additional features can introduce additional latency into your system. Always try to isolate additional functionality from the mining itself. Don't look onto PPLNS or PROP pools with charming graphs or vardiff support, they can handle that at the expense of miners... Unlike them, you're not able to drop the expensions on the hands of your users, with PPS every additional risk is balanced through your pocket.

If it's possible to live without heavy charts engine then it's better to do that. If it's possible to minimize risk of orphan blocks through truncation of transaction list then it's better to do that as well. Additional risks are costly, an ability to earn some +0.1 ETH per block in exchange for 12% risk of uncles is not worthly.

