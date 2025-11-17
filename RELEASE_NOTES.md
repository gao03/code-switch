# Code Switch v0.1.7

## 更新亮点
- ♻️ **cc-switch 导入更智能**：支持解析 `nmodel_provider` 以及 provider 内的 `name` 字段，即便 TOML 中使用别名也能正确识别 Codex 供应商；成功导入后按钮自动隐藏。
- 🧩 **首发 provider 不再回弹**：删除 Codex 供应商后不会再被默认配置覆盖，确保用户自定义列表持久生效。
- 🧠 **技能仓库 UI 修复**：技能仓库表单输入框拉伸、布局收敛，弹层视觉与深浅色模式更协调。
- 🐧 **Linux 包产物**：发布脚本新增 AppImage 与 `.deb` 构建，会自动写入版本号并上传到 Release 资产。

# Code Switch v0.1.6

## 更新亮点
- 🧠 **Claude Code Skill 管理**：新增技能页面可浏览、安装与卸载 Claude Code Skills，并在同一对话框中维护自定义技能仓库，方便按需扩展技能来源。
- 🪟 **窗口管理修复**：macOS/Windows 托盘切换到主窗口时会自动聚焦并解除最小化，仅首开时居中，避免频繁重置窗口位置；Windows 还会暂时启用置顶确保焦点正确。
- 📥 **cc-switch 配置导入**：主页提供 cc-switch 导入按钮，自动读取 `~/.cc-switch/config.json` 中尚未同步的供应商与 MCP 服务器，导入完成后按钮自动隐藏，避免重复操作。
