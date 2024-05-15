# 实时

FrankenPHP 带有一个内置的 Mercure Hub！
Mercure 允许将事件实时推送到所有连接的设备：它们将立即收到 JavaScript 事件。

无需 JS 库或 SDK！

![Mercure](../mercure-hub.png)

要启用 Mercure Hub，请按照 [Mercure 网站](https://mercure.rocks/docs/hub/config) 中的说明更新 `Caddyfile`。

要从您的代码中推送 Mercure 更新，我们推荐 [Symfony Mercure Component](https://symfony.com/components/Mercure)（不需要 Symfony 框架来使用）。
