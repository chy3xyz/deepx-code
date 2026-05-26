# deepx-code 演示录制脚本(vhs)
#
# 用法:
#   brew install vhs          # macOS;其它平台见 https://github.com/charmbracelet/vhs
#   vhs scripts/demo.tape     # 在仓库根目录跑,产出 assets/demo.gif
#
# 注意:演示会真实启动 deepx 并发起一次模型调用(用 ~/.deepx/model.yaml 的 key),
# 会消耗少量 flash token。Sleep 时长按本机/网络延迟可能需要微调——若 GIF 录到
# “半截”状态,调大下面发送后的 Sleep。

Output assets/demo.gif

Set Shell bash
Set FontSize 16
Set Width 1200
Set Height 720
Set Padding 18
Set Theme "Catppuccin Mocha"

# 在一个干净的小项目里启动(避免加载旧会话历史),等 TUI + codegraph 预热(不录这段)
Hide
Type "cd /tmp/deepx-demo && deepx"
Enter
Sleep 4s
Show

# 输入一个会触发 CodeGraph 的问题,展示:footer 活动行(思考→调用工具)+ 完成行
Sleep 800ms
Type "Greet 函数在哪定义、被谁调用?用 codegraph 查"
Sleep 600ms
Enter

# 等模型 + 工具 + 回答(延迟不稳,留足缓冲;录到半截就调大)
Sleep 14s

# 停在“✓ 完成”那一帧
Sleep 2500ms
