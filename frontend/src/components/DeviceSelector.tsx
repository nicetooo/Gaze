import React from 'react';
import { Select, Button, Space, Tooltip, Divider } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

const { Option } = Select;

interface Device {
  id: string;
  model: string;
  brand: string;
}

interface DeviceSelectorProps {
  devices: Device[];
  selectedDevice: string;
  onDeviceChange: (value: string) => void;
  onRefresh: () => void;
  loading?: boolean;
  style?: React.CSSProperties;
}

const DeviceSelector: React.FC<DeviceSelectorProps> = ({
  devices,
  selectedDevice,
  onDeviceChange,
  onRefresh,
  loading = false,
  style = {}
}) => {
  const { t } = useTranslation();
  return (
    <Select
      value={selectedDevice || undefined}
      onChange={onDeviceChange}
      style={{ width: 220, ...style }}
      placeholder={t("device_selector.placeholder")}
      dropdownRender={(menu) => (
        <>
          {menu}
          <Divider style={{ margin: '8px 0' }} />
          <div style={{ padding: '0 8px 4px' }}>
            <Button
              type="text"
              icon={<ReloadOutlined />}
              onClick={onRefresh}
              loading={loading}
              style={{ width: '100%', textAlign: 'left' }}
            >
              {t("device_selector.refresh")}
            </Button>
          </div>
        </>
      )}
    >
      {devices.map((d) => (
        <Option key={d.id} value={d.id}>
          {d.brand ? `${d.brand} ${d.model}` : d.model || d.id}
        </Option>
      ))}
    </Select>
  );
};

export default DeviceSelector;

