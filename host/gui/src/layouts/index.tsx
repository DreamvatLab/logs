import { Outlet } from 'umi';
import "antd/dist/reset.css"
import { ConfigProvider, theme } from 'antd';

export default function Layout() {
  return (
    <ConfigProvider theme={{ algorithm: theme.compactAlgorithm }}>
      <Outlet />
    </ConfigProvider>
  );
}
