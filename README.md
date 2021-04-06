# qtr

Quantitative trading robot

- qtr
- grid
- qst
- qsnap
- qresearch
- squeeze
- wsq
- qcandle

## `qtr`命令功能列表

**子命令**

- `grid` 网格交易  
  不带子命令时，根据配置文件制定的参数，启动网格交易。如果已经启动过，停止后再次启动，会读取`mongodb`的缓存。
    - print 打印网格状态
    - rebalance 自动再平衡  
      支持参数
        - dry-run 只打印不执行，用于确定需要的币量
    - clear
- `super` 超级趋势  
  使用超级趋势指标(`SuperTrend`)进行交易，不含风险管理。  
  主要用于针对芝麻开门(Gate)的`RESTful`接口进行现货交易。
- `mgrid`, `mg` 双向网格交易，主要针对的是3L3S
- `turtle` 海龟交易
- `sniper`
- `rtm`
- `ta` `TA`指标  
  主要用于验证TA指标的正确性和适用性。
- `scan` 按指定指标扫描指定币种列表，用于分析投资机会。
- `history`
- `profit`
- `snapshot`

## `grid`

## `qst`

## `qsnap`

## `qresearch`

## `squeeze`

## `wsq`

**全局参数**

