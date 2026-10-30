import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert } from 'antd';
import { CloseOutlined } from '@ant-design/icons';

import './TmsBanner.css';

const DISMISS_KEY = 'tmsBannerDismissed';
const DOWNLOAD_URL = 'https://3yuedaohang.com/tms-download.html?ref=xui';

function readDismissed(): boolean {
  try {
    return localStorage.getItem(DISMISS_KEY) === '1';
  } catch {
    return false;
  }
}

export default function TmsBanner() {
  const { t } = useTranslation();
  const [dismissed, setDismissed] = useState(readDismissed);

  if (dismissed) return null;

  function onClose() {
    try {
      localStorage.setItem(DISMISS_KEY, '1');
    } catch {
      // storage unavailable: hide for this page view only
    }
    setDismissed(true);
  }

  return (
    <Alert
      className="tms-banner"
      type="info"
      showIcon
      closable={{ closeIcon: <CloseOutlined />, onClose }}
      title={
        <span>
          {t('tmsBanner.text')}{' '}
          <a href={DOWNLOAD_URL} target="_blank" rel="noopener noreferrer">
            {t('tmsBanner.download')}
          </a>
        </span>
      }
    />
  );
}
