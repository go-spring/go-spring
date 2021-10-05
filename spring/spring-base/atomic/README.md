# atomic

封装标准库 atomic 包的操作函数。

## 方法列表

<table>
    <tr>
        <th>类型名</th>
        <th>方法名</th>
        <th>功能</th>
    </tr>
    <tr>
        <td rowspan="5">Int32</td>
         <td>Add</td>
        <td>wrapper for atomic.AddInt32.</td>
    </tr>
    <tr>
        <td>Store</td>
        <td>wrapper for atomic.StoreInt32.</td>
    </tr>
    <tr>
        <td>Load</td>
        <td>wrapper for atomic.LoadInt32.</td>
    </tr>
    <tr>
        <td>Swap</td>
        <td>wrapper for atomic.SwapInt32.</td>
    </tr>
    <tr>
        <td>CompareAndSwap</td>
        <td>wrapper for atomic.CompareAndSwapInt32.</td>
    </tr>
    <tr>
        <td rowspan="5">Int64</td>
         <td>Add</td>
        <td>wrapper for atomic.AddInt64.</td>
    </tr>
    <tr>
        <td>Load</td>
        <td>wrapper for atomic.LoadInt64.</td>
    </tr>
    <tr>
        <td>Store</td>
        <td>wrapper for atomic.StoreInt64.</td>
    </tr>
    <tr>
        <td>Swap</td>
        <td>wrapper for atomic.SwapInt64.</td>
    </tr>
    <tr>
        <td>CompareAndSwap</td>
        <td>wrapper for atomic.CompareAndSwapInt64.</td>
    </tr>
    <tr>
        <td rowspan="5">Uint32</td>
         <td>Add</td>
        <td>wrapper for atomic.AddUint32</td>
    </tr>
    <tr>
        <td>Load</td>
        <td>wrapper for atomic.LoadUint32.</td>
    </tr>
    <tr>
        <td>Store</td>
        <td>wrapper for atomic.StoreUint32.</td>
    </tr>
    <tr>
        <td>Swap</td>
        <td>wrapper for atomic.SwapUint32.</td>
    </tr>
    <tr>
        <td>CompareAndSwap</td>
        <td>wrapper for atomic.CompareAndSwapUint32.</td>
    </tr>
    <tr>
        <td rowspan="5">Uint64</td>
         <td>Add</td>
        <td>wrapper for atomic.AddUint64.</td>
    </tr>
    <tr>
        <td>Load</td>
        <td>wrapper for atomic.LoadUint64.</td>
    </tr>
    <tr>
        <td>Store</td>
        <td>wrapper for atomic.StoreUint64.</td>
    </tr>
    <tr>
        <td>Swap</td>
        <td>wrapper for atomic.SwapUint64.</td>
    </tr>
    <tr>
        <td>CompareAndSwap</td>
        <td>wrapper for atomic.CompareAndSwapUint64.</td>
    </tr>
    <tr>
        <td rowspan="5">Uintptr</td>
         <td>Add</td>
        <td>wrapper for atomic.AddUintptr.</td>
    </tr>
    <tr>
        <td>Load</td>
        <td>wrapper for atomic.LoadUintptr.</td>
    </tr>
    <tr>
        <td>Store</td>
        <td>wrapper for atomic.StoreUintptr.</td>
    </tr>
    <tr>
        <td>Swap</td>
        <td>wrapper for atomic.SwapUintptr.</td>
    </tr>
    <tr>
        <td>CompareAndSwap</td>
        <td>wrapper for atomic.CompareAndSwapUintptr.</td>
    </tr>
    <tr>
        <td rowspan="4">Pointer</td>
         <td>Load</td>
        <td>wrapper for atomic.LoadPointer.</td>
    </tr>
    <tr>
        <td>Store</td>
        <td>wrapper for atomic.StorePointer.</td>
    </tr>
    <tr>
        <td>Swap</td>
        <td>wrapper for atomic.SwapPointer.</td>
    </tr>
    <tr>
        <td>CompareAndSwap</td>
        <td>wrapper for atomic.CompareAndSwapPointer.</td>
    </tr>
    <tr>
        <td rowspan="2">Value</td>
         <td>Load</td>
        <td>returns the value set by the most recent Store.</td>
    </tr>
    <tr>
        <td>Store</td>
        <td>sets the value of the Value to x.</td>
    </tr>
</table>
