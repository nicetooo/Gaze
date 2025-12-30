import { ThemeConfig, theme } from 'antd';

const { defaultAlgorithm, darkAlgorithm } = theme;

/**
 * Ant Design Theme Configuration for Native macOS Appearance
 * 
 * This theme customizes Ant Design components to match macOS design guidelines:
 * - Compact component sizes (28px height vs default 32px)
 * - macOS system blue (#007AFF) as primary color
 * - Subtle borders and shadows
 * - Reduced border radius (6px)
 * - System font stack
 * - Component-specific refinements
 */

export const getTheme = (mode: 'light' | 'dark'): ThemeConfig => {
  const isDark = mode === 'dark';
  
  return {
    algorithm: isDark ? darkAlgorithm : defaultAlgorithm,
    token: {
      // ===== Colors: System Palette =====
      colorPrimary: '#007AFF', // System Blue
      colorInfo: '#007AFF', 
      colorSuccess: '#34C759', // System Green
      colorWarning: '#FF9500', // System Orange
      colorError: '#FF3B30',   // System Red
      
      // Dynamic Colors based on Mode
      colorTextBase: isDark ? '#ffffff' : '#000000',
      colorBgBase: isDark ? '#1e1e1e' : '#ffffff',
      colorBgContainer: isDark ? '#1e1e1e' : '#ffffff',
      colorBgElevated: isDark ? '#2c2c2e' : '#ffffff',
      colorBgLayout: isDark ? '#000000' : '#f5f5f7', // Dark mode uses black background often, sidebar is lighter
      
      // ===== Colors: Borders & Separators =====
      colorBorder: isDark ? 'rgba(255, 255, 255, 0.12)' : 'rgba(0, 0, 0, 0.12)',
      colorBorderSecondary: isDark ? 'rgba(255, 255, 255, 0.08)' : 'rgba(0, 0, 0, 0.08)',
      colorSplit: isDark ? 'rgba(255, 255, 255, 0.08)' : 'rgba(0, 0, 0, 0.06)',

      // ===== Typography =====
      fontFamily: `-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'Helvetica Neue', sans-serif`,
      fontSize: 13, // Standard macOS text size
      
      // ===== Sizes & Spacing =====
      controlHeight: 28, // Compact native feel
      controlHeightSM: 24,
      controlHeightLG: 32, // Large controls shouldn't be too huge
      
      borderRadius: 6,
      borderRadiusSM: 4,
      borderRadiusLG: 8,
      borderRadiusXS: 2,

      // ===== Interactive & States =====
      controlOutline: 'rgba(0, 122, 255, 0.24)', // Focus ring
      controlOutlineWidth: 3,
      
      // ===== Motion =====
      motionDurationFast: '0.1s',
      motionDurationMid: '0.2s',
      motionDurationSlow: '0.3s',
    },
    
    components: {
      // ===== Layout =====
      Layout: {
        bodyBg: isDark ? '#1e1e1e' : '#ffffff',
        siderBg: isDark ? '#2C2C2E' : '#F5F5F7', // macOS Dark Sidebar
        headerBg: isDark ? '#1e1e1e' : '#ffffff',
      },

      // ===== Navigation: Menu =====
      Menu: {
        itemHeight: 30, // Compact menu items
        itemBorderRadius: 6,
        itemMarginInline: 8, // macOS style spacing
        subMenuItemBg: 'transparent',
        activeBarBorderWidth: 0, // No border bar
        
        // macOS Sidebar Style (Finder-like):
        // Active state: System Blue background, White text
        itemSelectedBg: '#007AFF', 
        itemSelectedColor: '#ffffff',
        
        // Hover state
        itemHoverBg: isDark ? 'rgba(255, 255, 255, 0.05)' : 'rgba(0, 0, 0, 0.04)',
        
        iconSize: 15,
        fontSize: 13,
        itemColor: isDark ? 'rgba(255, 255, 255, 0.85)' : 'rgba(0, 0, 0, 0.88)',
      },

      // ===== Inputs & Forms =====
      Input: {
        controlHeight: 28,
        borderRadius: 5,
        paddingBlock: 3,
        paddingInline: 8,
        activeBorderColor: '#007AFF',
        hoverBorderColor: isDark ? 'rgba(255, 255, 255, 0.2)' : 'rgba(0, 0, 0, 0.2)',
        activeShadow: '0 0 0 3px rgba(0, 122, 255, 0.15)',
        colorBgContainer: isDark ? 'rgba(255, 255, 255, 0.1)' : '#ffffff', // Transparent-ish background in dark mode commonly
      },
      Select: {
        controlHeight: 28,
        borderRadius: 5,
        selectorBg: isDark ? 'rgba(255, 255, 255, 0.1)' : '#ffffff',
        optionSelectedBg: '#007AFF',
        optionSelectedColor: '#ffffff',
        optionPadding: '4px 12px',
      },
      Button: {
        controlHeight: 28,
        contentFontSize: 13,
        borderRadius: 5,
        defaultShadow: isDark ? 'none' : '0 1px 0 rgba(0, 0, 0, 0.02)',
        primaryShadow: '0 1px 0 rgba(0, 0, 0, 0.1)',
        paddingInline: 12,
        fontWeight: 400,
      },
      Checkbox: {
        borderRadiusSM: 3,
        controlInteractiveSize: 14,
      },
      Switch: {
        trackHeight: 20,
        handleSize: 16,
        trackMinWidth: 38,
        colorPrimary: '#34C759', // Green for "On" state usually
        trackPadding: 2,
      },
      
      // ===== Data Display =====
      Table: {
        headerBg: isDark ? '#2C2C2E' : '#f5f5f7', // Match layout background
        headerColor: isDark ? '#A0A0A0' : '#666666',
        headerBorderRadius: 0, 
        headerSplitColor: 'transparent',
        rowHoverBg: isDark ? 'rgba(255, 255, 255, 0.04)' : '#F0F5FF',
        cellPaddingBlock: 8,
        cellPaddingInline: 12,
        borderColor: isDark ? 'rgba(255, 255, 255, 0.05)' : 'rgba(0,0,0,0.06)',
      },
      List: {
        itemPadding: '8px 12px',
      },
      Tree: {
         directoryNodeSelectedBg: '#007AFF',
         directoryNodeSelectedColor: '#ffffff',
         nodeSelectedBg: 'rgba(0, 122, 255, 0.1)',
         titleHeight: 24,
      },
      
      // ===== Feedback & Overlays =====
      Modal: {
        headerBg: 'transparent',
        contentBg: isDark ? '#2C2C2E' : '#ffffff',
        boxShadow: '0 8px 30px rgba(0, 0, 0, 0.3)',
        borderRadiusLG: 10,
      },
      Message: {
        borderRadiusLG: 8,
        contentPadding: '8px 12px',
        contentBg: isDark ? 'rgba(60, 60, 60, 0.9)' : 'rgba(40, 40, 40, 0.85)',
      },
      Notification: {
        width: 360,
        borderRadiusLG: 10,
        paddingContentHorizontalLG: 16,
      },
      Tooltip: {
        colorBgSpotlight: isDark ? 'rgba(255, 255, 255, 0.9)' : 'rgba(40, 40, 40, 0.9)', // Inverse tooltip
        colorTextLightSolid: isDark ? '#000000' : '#ffffff',
        borderRadius: 4,
      },
      
      // ===== Other =====
      Segmented: {
        itemSelectedBg: isDark ? '#636366' : '#ffffff',
        trackBg: isDark ? 'rgba(118, 118, 128, 0.24)' : 'rgba(0, 0, 0, 0.05)',
        itemColor: isDark ? '#ffffff' : '#555555',
        itemSelectedColor: isDark ? '#ffffff' : '#000000',
        trackPadding: 2,
        borderRadius: 6,
        boxShadowTertiary: '0 1px 2px 0 rgba(0,0,0,0.04)',
      },
      Tabs: {
         cardBg: 'transparent',
         itemActiveColor: '#007AFF',
         itemSelectedColor: '#007AFF',
         inkBarColor: '#007AFF',
         titleFontSize: 13,
      },
      Tag: {
        borderRadiusSM: 4,
        defaultBg: isDark ? 'rgba(255, 255, 255, 0.08)' : 'rgba(0, 0, 0, 0.04)',
        defaultColor: isDark ? '#ffffff' : '#1d1d1f',
      }
    },
  };
};
