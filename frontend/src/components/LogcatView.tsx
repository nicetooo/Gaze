import { useRef, useEffect } from 'react';
import { Button, Input, Select, Space } from 'antd';
import { PauseOutlined, PlayCircleOutlined, ClearOutlined, DownOutlined } from '@ant-design/icons';
import { useVirtualizer } from '@tanstack/react-virtual';
// @ts-ignore
import { main } from '../../wailsjs/go/models';

const { Option } = Select;

interface Device {
  id: string;
  state: string;
  model: string;
  brand: string;
}

interface LogcatViewProps {
  devices: Device[];
  selectedDevice: string;
  setSelectedDevice: (device: string) => void;
  packages: main.AppPackage[];
  selectedPackage: string;
  setSelectedPackage: (pkg: string) => void;
  isLogging: boolean;
  toggleLogcat: () => void;
  logs: string[];
  setLogs: (logs: string[]) => void;
  logFilter: string;
  setLogFilter: (filter: string) => void;
  autoScroll: boolean;
  setAutoScroll: (scroll: boolean) => void;
}

export default function LogcatView({
  devices,
  selectedDevice,
  setSelectedDevice,
  packages,
  selectedPackage,
  setSelectedPackage,
  isLogging,
  toggleLogcat,
  logs,
  setLogs,
  logFilter,
  setLogFilter,
  autoScroll,
  setAutoScroll,
}: LogcatViewProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const scrollingRef = useRef(false);

  const virtualizer = useVirtualizer({
    count: logs.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 24,
    overscan: 10,
  });

  // 自动滚动逻辑
  useEffect(() => {
    if (autoScroll && logs.length > 0 && !scrollingRef.current) {
      virtualizer.scrollToIndex(logs.length - 1, {
        align: 'end',
      });
    }
  }, [logs.length, autoScroll, virtualizer]);

  const handleScroll = (e: React.UIEvent<HTMLDivElement>) => {
    // 如果正在进行点击后的自动平滑滚动，不处理滚动事件，防止干扰 autoScroll 状态
    if (scrollingRef.current) return;

    const target = e.currentTarget;
    const { scrollTop, scrollHeight, clientHeight } = target;
    
    // 如果日志还在刷新，我们加宽判断范围（150px）
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 150;
    
    if (isAtBottom) {
      if (!autoScroll) setAutoScroll(true);
    } else {
      // 只有在用户明显向上滚动时才停掉 autoscroll
      if (autoScroll) setAutoScroll(false);
    }
  };

  const scrollToBottom = () => {
    scrollingRef.current = true;
    setAutoScroll(true);
    
    virtualizer.scrollToIndex(logs.length - 1, {
      align: 'end',
      behavior: 'smooth',
    });

    // 动画结束后释放锁定，允许后续的手动滚动检测
    setTimeout(() => {
      scrollingRef.current = false;
    }, 1000); // 增加锁定时长以确保平滑滚动执行完毕
  };

  const getLogColor = (text: string) => {
    // Standard ADB format matches like " D/", " I/", " W/", " E/" or starting with them
    if (text.includes(' E/') || text.includes(' F/') || text.startsWith('E/')) return '#f14c4c'; // Error / Fatal
    if (text.includes(' W/') || text.startsWith('W/')) return '#cca700'; // Warning
    if (text.includes(' I/') || text.startsWith('I/')) return '#3794ff'; // Info
    if (text.includes(' D/') || text.startsWith('D/')) return '#4ec9b0'; // Debug
    return '#d4d4d4'; // Verbose / Default
  };

  const renderLogLine = (text: string) => {
    const color = getLogColor(text);
    
    if (!logFilter) {
      return <span style={{ color }}>{text}</span>;
    }

    const parts = text.split(new RegExp(`(${logFilter})`, 'gi'));
    return (
      <span style={{ color }}>
        {parts.map((part, i) => 
          part.toLowerCase() === logFilter.toLowerCase() ? (
            <mark key={i} style={{ backgroundColor: '#ffcc00', color: '#000', borderRadius: '2px', padding: '0 1px' }}>
              {part}
            </mark>
          ) : (
            <span key={i}>{part}</span>
          )
        )}
      </span>
    );
  };

  return (
    <div style={{ padding: '16px 24px', flex: 1, display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexShrink: 0 }}>
        <h2 style={{ margin: 0 }}>Logcat</h2>
        <Space>
          <Select 
            value={selectedDevice} 
            onChange={setSelectedDevice} 
            style={{ width: 180 }} 
            placeholder="Select Device"
            disabled={isLogging}
          >
            {devices.map(d => (
              <Option key={d.id} value={d.id}>
                {d.brand ? `${d.brand} ${d.model}` : (d.model || d.id)}
              </Option>
            ))}
          </Select>
          <Select
            showSearch
            value={selectedPackage}
            onChange={setSelectedPackage}
            style={{ width: 220 }}
            placeholder="All Apps (Optional)"
            disabled={isLogging}
            allowClear
            filterOption={(input, option) =>
              (option?.children as unknown as string).toLowerCase().indexOf(input.toLowerCase()) >= 0
            }
          >
            {packages.map(p => (
              <Option key={p.name} value={p.name}>{p.name}</Option>
            ))}
          </Select>
          <Button 
            type={isLogging ? 'primary' : 'default'} 
            danger={isLogging}
            icon={isLogging ? <PauseOutlined /> : <PlayCircleOutlined />} 
            onClick={toggleLogcat}
          >
            {isLogging ? 'Stop' : 'Start'}
          </Button>
          <Button icon={<ClearOutlined />} onClick={() => setLogs([])}>
            Clear
          </Button>
        </Space>
      </div>
      
      <Input 
        placeholder="Filter logs..." 
        value={logFilter}
        onChange={e => setLogFilter(e.target.value)}
        style={{ marginBottom: 12, flexShrink: 0 }}
      />

      <div style={{ flex: 1, position: 'relative', minHeight: 0, backgroundColor: '#1e1e1e', borderRadius: '4px', overflow: 'hidden' }}>
        <div
          ref={parentRef}
          onScroll={handleScroll}
          style={{
            height: '100%',
            overflow: 'auto',
          }}
        >
          <div
            style={{
              height: `${virtualizer.getTotalSize()}px`,
              width: '100%',
              position: 'relative',
            }}
          >
            {virtualizer.getVirtualItems().map((virtualItem) => (
              <div
                key={virtualItem.index}
                ref={virtualizer.measureElement}
                data-index={virtualItem.index}
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  transform: `translateY(${virtualItem.start}px)`,
                  padding: '2px 12px',
                  borderBottom: '1px solid #2d2d2d',
                  color: '#d4d4d4',
                  fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                  fontSize: '12px',
                  lineHeight: '1.5',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                }}
              >
                {renderLogLine(logs[virtualItem.index])}
              </div>
            ))}
          </div>
        </div>
        
        {!autoScroll && logs.length > 0 && (
          <Button
            type="primary"
            shape="circle"
            icon={<DownOutlined />}
            size="large"
            onClick={scrollToBottom}
            style={{
              position: 'absolute',
              bottom: 24,
              right: 24,
              boxShadow: '0 4px 12px rgba(0, 0, 0, 0.4)',
              zIndex: 100,
            }}
          />
        )}
      </div>
    </div>
  );
}
