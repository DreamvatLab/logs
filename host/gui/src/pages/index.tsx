import { LogEntry } from "@/models";
import HomeVM from "@/vms/homeVM";
import { App, Button, DatePicker, Input, InputNumber, Modal, Select, Space, Switch, Table, TableColumnType, TableColumnsType, Typography } from "antd";
import dayjs from "dayjs";
import { InfoCircleOutlined } from '@ant-design/icons';
import { observer } from "mobx-react";
import ReactJson from 'react-json-view'

const { Text } = Typography;

function truncateStr(str: string, { length = 100 }: { length?: number } = {}) {
  if (!str) {
    return str;
  }

  if (str.length <= length) {
    return str;
  } else {
    return str.slice(0, length) + '...';
  }
}

const columns: TableColumnsType<LogEntry> = [
  {
    title: 'Level',
    render: (_: any, x: LogEntry) => {
      switch (x.Level) {
        case 0:
          return <Text type="secondary">Verbose</Text>;
        case 1:
          return <Text type="secondary">Debug</Text>;
        case 2:
          return <Text>Info</Text>;
        case 3:
          return <Text type="warning">Warning</Text>;
        case 4:
          return <Text type="danger">Error</Text>;
        case 5:
          return <Text type="danger">Fatal</Text>;
        default:
          return <Text type="secondary">Unknown</Text>;
      }
    },
    width: 60,
    responsive: ['sm'],
  },
  {
    title: 'Message',
    dataIndex: 'Message',
    render: (_: any, x: LogEntry) => {
      const msg = truncateStr(x.Message, { length: 200 });

      switch (x.Level) {
        case 0:
          return <Text type="secondary">{msg}</Text>;
        case 1:
          return <Text type="secondary">{msg}</Text>;
        case 2:
          return <Text>{msg}</Text>;
        case 3:
          return <Text type="warning">{msg}</Text>;
        case 4:
          return <Text type="danger">{msg}</Text>;
        case 5:
          return <Text type="danger">{msg}</Text>;
        default:
          return <Text type="secondary">{msg}</Text>;
      }
    },
  },
  {
    title: 'User',
    dataIndex: 'User',
    // width: 300,
    responsive: ['lg'],
  },
  {
    title: 'TraceNo',
    dataIndex: 'TraceNo',
    align: "center",
    // width: 200,
    responsive: ['xl'],
  },
  {
    title: 'Date',
    align: "center",
    width: 160,
    render: (_: any, x: LogEntry) => dayjs(x.CreatedOnUtc).format("MM/DD/YYYY hh:mm:ss A"),
    responsive: ['md'],
  },
  {
    align: "center",
    width: 32,
    render: (_: any, x: LogEntry) => <a onClick={() => showDetails(x)}><InfoCircleOutlined /></a>,
  },
];

const showDetails = (x: LogEntry) => {
  const content = <ReactJson src={x} theme="monokai" iconStyle="circle" displayDataTypes={false} />;

  Modal.info({
    icon: null,
    closable: true,
    maskClosable: true,
    content: content,
    width: "100%",
  });
};

const ClientsDDL = observer(({ homeVM }: { homeVM: HomeVM }) => {
  const options = homeVM.clients.map(x => {
    return { label: x, value: x };
  });

  return <Select
    style={{ minWidth: 100 }}
    popupMatchSelectWidth={false}
    placeholder="Client"
    value={homeVM.client}
    onChange={(v) => homeVM.setClient(v)}
    options={options}
  />;
});

const DatabasesDDL = observer(({ homeVM }: { homeVM: HomeVM }) => {
  const options = homeVM.databases.map(x => {
    return { label: x, value: x };
  });

  return <Select
    style={{ minWidth: 100 }}
    popupMatchSelectWidth={false}
    placeholder="Database"
    value={homeVM.database}
    onChange={(v) => homeVM.setDatabase(v)}
    options={options}
  />;
});

const TablesDDL = observer(({ homeVM }: { homeVM: HomeVM }) => {
  const options = homeVM.tables.map(x => {
    return { label: x, value: x };
  });

  return <Select
    style={{ minWidth: 70 }}
    popupMatchSelectWidth={false}
    placeholder="Table"
    value={homeVM.table}
    onChange={(v) => homeVM.setTable(v)}
    options={options}
  />;
});

const LevelsDDL = observer(({ homeVM }: { homeVM: HomeVM }) => {
  return <Select
    style={{ minWidth: 70 }}
    popupMatchSelectWidth={false}
    placeholder="Level"
    value={homeVM.level}
    onChange={(v) => homeVM.setLevel(v)}
    options={homeVM.levels}
  />;
});

const MessageInput = observer(({ homeVM }: { homeVM: HomeVM }) => {
  return <Input placeholder="Message" value={homeVM.message} onChange={(e) => homeVM.setMessage(e.target.value)} />;
});

const UserInput = observer(({ homeVM }: { homeVM: HomeVM }) => {
  return <Input placeholder="User" value={homeVM.user} onChange={(e) => homeVM.setUser(e.target.value)} />;
});

const TraceNoInput = observer(({ homeVM }: { homeVM: HomeVM }) => {
  return <Input placeholder="TraceNo" value={homeVM.traceNo} onChange={(e) => homeVM.setTraceNo(e.target.value)} />;
});

const FlagsInput = observer(({ homeVM }: { homeVM: HomeVM }) => {
  return <InputNumber placeholder="Flags" precision={0} min={0} value={homeVM.flags} onChange={(v) => homeVM.setFlags(v ?? undefined)} style={{ width: 50 }} />;
});

const DateRangePicker = observer(({ homeVM }: { homeVM: HomeVM }) => {
  return <DatePicker.RangePicker allowEmpty={[true, true]} value={homeVM.dateRange} onChange={(v) => homeVM.setDateRange(v)} />;
});

// const FuzzySearchSwitch = observer(({ homeVM }: { homeVM: HomeVM }) => {
//   return <Switch checkedChildren="Fuzzy" unCheckedChildren="Fuzzy" checked={homeVM.fuzzySearch} onChange={(v) => homeVM.setFuzzySearch(v)} />;
// });

const LogTable = observer(({ homeVM }: { homeVM: HomeVM }) => {
  return <Table rowKey="ID"
    size="small"
    columns={columns}
    loading={homeVM.loading}
    scroll={{
      x: true,
    }}
    pagination={{ total: homeVM.total, current: homeVM.pageIndex, pageSize: homeVM.pageSize, onChange: (v) => homeVM.setPageIndex(v) }}
    dataSource={homeVM.data}
  />;
});

const HomePage: React.FC = () => {
  const homeVM = new HomeVM();
  homeVM.init();

  return (
    <>
      <Space>
        <ClientsDDL homeVM={homeVM} />
        <DatabasesDDL homeVM={homeVM} />
        <TablesDDL homeVM={homeVM} />
        <LevelsDDL homeVM={homeVM} />
        <MessageInput homeVM={homeVM} />
        <UserInput homeVM={homeVM} />
        <TraceNoInput homeVM={homeVM} />
        <FlagsInput homeVM={homeVM} />
        <DateRangePicker homeVM={homeVM} />
        {/* <FuzzySearchSwitch homeVM={homeVM} /> */}
        <Button type="primary" onClick={() => homeVM.search()}>Search</Button>
        <Button onClick={() => homeVM.reset()}>Reset</Button>
      </Space>
      <LogTable homeVM={homeVM} />
    </>
  );
}

export default HomePage;
