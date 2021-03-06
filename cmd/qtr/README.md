# qtr

qtr = quantitative trading robot，量化交易机器人。

qtr下放的子命令多为快速实验中的，较稳定的命令会逐步移出去，防止在快速迭代的过程中受到不必要的干扰。

稳定指的是两种情况：第一，使用场景开始固化；第二，策略基本没有大幅改动了，只剩下参数调优。

## 网格交易

**`print`** 打印网格

```shell
./bin/qtr -c config/config.json grid print
```

**`rebalance`** 自动平衡

```shell
./bin/qtr -c config/config.json grid rebalance --dry-run
```

**`grid`** 运行网格

```shell
./bin/qtr -c config/config.json grid
```

**`clear`** 清空网格并取消订单

```shell
./bin/qtr -c config/config.json grid clear
```
