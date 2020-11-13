# qresearch使用说明

```
./qresearch.0.2.0 -h
NAME:
   qresearch.0.2.0 - test kline data for specific strategy

USAGE:
   qresearch.0.2.0 [global options] command [command options] [arguments...]

VERSION:
   0.2.0

COMMANDS:
   super    Trading with grid strategy (RESTful API)
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --config file, -c file  load configuration from file (default: "config.json")
   --help, -h              show help (default: false)
   --version, -v           print the version (default: false)
```

配置

配置文件中仅需要指定日志位置和级别。
```json
{
  "log": {
    "level": "debug",
    "outputs": [
      "log/qresearch_output.log"
    ],
    "errors": [
      "log/qresearch_error.log"
    ]
  }
}
```

命令
- `super`: `SuperTrend`参数调优

## `super`

**`SuperTrend`参数调优**

针对同一份数据（同一币种，同一区间），逐步调整`SuperTrend`参数，输出收益情况。用于观察参数对收益的影响，从中选择最优组合。

通过借助`matplotlib`画图，可以快速找出最优的参数组合。

## `window`

通过移动窗口，查看`SuperTrend`策略在不同时间段的表现。

如果收益率忽升忽降变化很大，说明收到了个别交易的影响太大，该策略的参数组合，就是不稳定的。