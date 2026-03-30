1. 配置文件使用 yml 格式
2. 程序标配 makefile，支持docker 构建和 docker-compose 部署
3. 主程序使用 golang 进行编写，前端使用 vue，风格以绿色为主题色，要求具备科技感和扁平风格。前端图表使用 lightweight 插件。
4. 前端显示以及消息通知的时间，统一使用 utc+8 的时区
5. 所有的需求放在 pdms 目录
6. 本地调试只有数据库用 docker 在运行，后端启动用 make backend，前端启动用 make frontend
