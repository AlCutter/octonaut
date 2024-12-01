# Octonaut

Octonaut is a tool for modeling electricity usage and costs for Octopus Energy customers.

This tool uses the Octopus Energy API to fetch your electricity consumption data and can use it calculate what your historical usage would have cost you under different Octoput tariffs.

It also supports a rudimentary model of a battery energy storage system (BESS), which can give you an idea of what you may have paid for your historical consumption with such a system installed for an arbitrary Octopus tariff.

## Usage

First, you'll need to get your Octopus account number (it looks something like `A-1111ABC2D`), and your Octopus API key which you can get from your octopus dashboard under personal details` > `developer settings` > `API access`, here: https://octopus.energy/dashboard/new/accounts/personal-details/api-access

Your API key should look like a long string of random characters starting with `sk_live_`.

Octonaut is a command-line tool, and can be run like so, substituting your account number and API key:

```
$ go run github.com/AlCutter/octonaut/cmd/octonaut --account=A-1111ABCD2D --key=sk_live_...
```

Octonaut has 3 commands:

`sync`: This downloads your historical electricity consumption data, and stores it locally to save unduly sending too many requests to Octopus' servers.

`products`: This lists all the Octopus electricity tariffs.

`model`: This command does cost calculations based on your historical consumption for different hypothetical tariff and battery configurations.

### Examples

First let Octonaut sync your consumption data locally (by default it'll store this in a file called `octonaut.sqlite3` in the current directory):

```bash
$ go run github.com/AlCutter/octonaut/cmd/octonaut --account=A-1111ABCD2D --key=sk_live_... sync
8:44PM INFO Syncing A-1111ABCD2D
8:44PM INFO  + Syncing property 1234567
8:44PM INFO  | + Syncing MPAN 1234567890123
8:44PM INFO  | | + Syncing Meter 12A1234567
8:44PM INFO  | | | + Syncing Consumption since 0001-01-01 00:00:00 +0000 UTC
8:44PM INFO  | | | | Got 12345 records
```

Now that Octonaut has your consumption data locally, it can answer questions about it for you, for example: How much would I have paid if I were on the Agile tariff?

```bash
$ go run github.com/AlCutter/octonaut/cmd/octonaut --account=A-1111ABCD2D --key=sk_live_... model --from=2024-01-01 --tariff=AGILE-23-12-06
8:53PM INFO From: 2024-01-01 00:00:00 +0000 UTC
8:53PM INFO To: 2024-12-01 00:00:00 +0000 GMT
8:53PM WARN Missing data between 2024-06-07 09:30:00 +0000 UTC and 2024-06-07 12:30:00 +0000 UTC, inserting zero usage intervals
8:53PM WARN Missing data between 2024-08-05 21:30:00 +0000 UTC and 2024-08-06 13:30:00 +0000 UTC, inserting zero usage intervals
8:53PM INFO 16080 ConsumptionIntervals
8:53PM INFO 16080 TariffRates
8:53PM INFO Energy    : £1234.56 (inc. VAT) (12345.67 kWh)
8:53PM INFO Standing  : £123.45 (inc. VAT) (335.0 days)
8:53PM INFO Total Cost: £2345.67 (£12.34/day, effective £0.12/kWh)
```

You can look at other currently available tariff codes using the `products` command:

```bash
$ go run ./cmd/octonaut --account=A-1111ABCD2D --key=sk_live_... products
8:57PM INFO AGILE-24-10-01:
8:57PM INFO   With Agile Octopus, you get access to half-hourly energy prices, tied to wholesale prices and updated daily.  The unit rate is capped at 100p/kWh (including VAT).
8:57PM INFO AGILE-BB-24-10-01:
8:57PM INFO   With Agile Octopus, you get access to half-hourly energy prices, tied to wholesale prices and updated daily.  The unit rate is capped at 100p/kWh (including VAT).
8:57PM INFO AGILE-OUTGOING-19-05-13:
8:57PM INFO   Outgoing Octopus Agile rate pays you for all your exported energy based on the day-ahead wholesale rate.
8:57PM INFO AGILE-OUTGOING-BB-23-02-28:
8:57PM INFO   Outgoing Octopus Agile rate pays you for all your exported energy based on the day-ahead wholesale rate.
8:57PM INFO COOP-FIX-12M-24-11-28:
8:57PM INFO   This fixed tariff locks in your unit rates and standing charges for 12 months with no exit fees.
...
```

You can also ask octonaut to calculate what your bill might have looked like if you had a residential battery installed in order to to _load shift_ your consumption.
You'll need to tell it what capacity of battery in kWh, how quickly it can charge in kW, and between which times it should charge:

```bash
$ go run ./cmd/octonaut --account=A-11111ABCD2D --key=sk_live_.... model --from=2024-01-01 --tariff=GO-VAR-22-10-14 --battery_capacity=40 --battery_rate=10 --battery_charge="23.5-4.5"
```

## Caveats

This software is work-in-progress, and apart from any unintended bugs and errors, also contains some known limitations. E.g. it currently assumes you like in the `J` post-code area.

Pull requests, issues, etc. all very welcome!

