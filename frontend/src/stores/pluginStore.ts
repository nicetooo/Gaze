import { create } from "zustand";
import {
  ListPlugins,
  GetPlugin,
  SavePlugin,
  DeletePlugin,
  TogglePlugin,
  TestPlugin,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";

/**
 * Plugin data model (complete plugin object from backend)
 * 
 * This represents the complete plugin as stored in the database and returned
 * by the backend API. It includes:
 * - metadata: Plugin configuration managed by the form (id, name, filters, etc.)
 * - sourceCode: User's TypeScript/JavaScript code
 * - language: Source code language
 * - compiledCode: Transpiled JavaScript for execution
 * 
 * @note This is different from the Plugin type in plugin.d.ts, which represents
 * the plugin object structure in user's code (without metadata, only logic functions)
 */
export interface Plugin {
  metadata: {
    id: string;
    name: string;
    version: string;
    author: string;
    description: string;
    enabled: boolean;
    filters: {
      sources: string[];
      types: string[];
      levels: string[];
      urlPattern: string;
      titleMatch: string;
    };
    config: Record<string, any>;
    createdAt: string;
    updatedAt: string;
  };
  sourceCode: string;
  language: "javascript" | "typescript";
  compiledCode: string;
}

export interface TestResult {
  derivedEvents: any[];
  tags: string[];
  metadata: Record<string, any>;
  error?: string;
}

interface PluginState {
  // 插件列表
  plugins: Plugin[];
  loading: boolean;
  error: string | null;

  // 当前编辑的插件
  currentPlugin: Plugin | null;
  editorOpen: boolean;
  editorInitialTab: string; // 编辑器打开时的初始 tab

  // 测试相关
  testResult: TestResult | null;
  testing: boolean;

  // 操作
  loadPlugins: () => Promise<void>;
  getPlugin: (id: string) => Promise<Plugin | null>;
  savePlugin: (plugin: Partial<Plugin>) => Promise<void>;
  deletePlugin: (id: string) => Promise<void>;
  togglePlugin: (id: string, enabled: boolean) => Promise<void>;
  testPlugin: (script: string, eventId: string) => Promise<TestResult>;

  // UI 操作
  openEditor: (plugin?: Plugin, initialTab?: string) => void;
  closeEditor: () => void;
  setCurrentPlugin: (plugin: Plugin | null) => void;
}

export const usePluginStore = create<PluginState>((set, get) => ({
  // 状态
  plugins: [],
  loading: false,
  error: null,
  currentPlugin: null,
  editorOpen: false,
  editorInitialTab: "basic", // 默认显示基础信息页
  testResult: null,
  testing: false,

  // 加载插件列表
  loadPlugins: async () => {
    set({ loading: true, error: null });
    try {
      const result = await ListPlugins();
      // ListPlugins 返回 PluginMetadata[]，需要转换为 Plugin[] 格式
      const plugins: Plugin[] = (result as any[]).map((metadata: any) => ({
        metadata: {
          id: metadata.id || "",
          name: metadata.name || "",
          version: metadata.version || "1.0.0",
          author: metadata.author || "",
          description: metadata.description || "",
          enabled: metadata.enabled !== undefined ? metadata.enabled : true,
          filters: metadata.filters || { sources: [], types: [], levels: [], urlPattern: "", titleMatch: "" },
          config: metadata.config || {},
          createdAt: metadata.createdAt || "",
          updatedAt: metadata.updatedAt || "",
        },
        sourceCode: "", // ListPlugins 不返回源码，需要时调用 GetPlugin
        language: "typescript" as const,
        compiledCode: "",
      }));
      set({ plugins, loading: false });
    } catch (error) {
      console.error("Failed to load plugins:", error);
      set({
        error: error instanceof Error ? error.message : "Failed to load plugins",
        loading: false,
      });
    }
  },

  // 获取单个插件
  getPlugin: async (id: string) => {
    set({ loading: true, error: null });
    try {
      const result = await GetPlugin(id);
      const plugin = result as any;
      // GetPlugin 返回完整的 Plugin 对象（已包含 metadata 嵌套结构）
      const pluginData: Plugin = {
        metadata: plugin.metadata || {},
        sourceCode: plugin.sourceCode || "",
        language: (plugin.language || "typescript") as "javascript" | "typescript",
        compiledCode: plugin.compiledCode || "",
      };
      set({ loading: false });
      return pluginData;
    } catch (error) {
      console.error("Failed to get plugin:", error);
      set({
        error: error instanceof Error ? error.message : "Failed to get plugin",
        loading: false,
      });
      return null;
    }
  },

  // 保存插件
  savePlugin: async (plugin: Partial<Plugin>) => {
    set({ loading: true, error: null });
    try {
      // 使用 Wails 生成的 PluginSaveRequest 类
      const req = new main.PluginSaveRequest({
        id: plugin.metadata?.id || "",
        name: plugin.metadata?.name || "",
        version: plugin.metadata?.version || "1.0.0",
        author: plugin.metadata?.author || "",
        description: plugin.metadata?.description || "",
        sourceCode: plugin.sourceCode || "",
        language: plugin.language || "typescript",
        compiledCode: plugin.compiledCode || plugin.sourceCode || "",
        filters: plugin.metadata?.filters || { sources: [], types: [], levels: [], urlPattern: "", titleMatch: "" },
        config: plugin.metadata?.config || {},
        // 透传 enabled，避免后端把禁用插件在保存后重置为启用
        enabled: plugin.metadata?.enabled,
      });

      await SavePlugin(req);
      await get().loadPlugins();
      set({ loading: false, editorOpen: false, currentPlugin: null });
    } catch (error) {
      console.error("Failed to save plugin:", error);
      set({
        error: error instanceof Error ? error.message : "Failed to save plugin",
        loading: false,
      });
      throw error;
    }
  },

  // 删除插件
  deletePlugin: async (id: string) => {
    set({ loading: true, error: null });
    try {
      await DeletePlugin(id);
      await get().loadPlugins();
      set({ loading: false });
    } catch (error) {
      console.error("Failed to delete plugin:", error);
      set({
        error: error instanceof Error ? error.message : "Failed to delete plugin",
        loading: false,
      });
      throw error;
    }
  },

  // 启用/禁用插件
  togglePlugin: async (id: string, enabled: boolean) => {
    try {
      await TogglePlugin(id, enabled);
      // 更新本地状态
      set((state) => ({
        plugins: state.plugins.map((p) =>
          p.metadata.id === id
            ? { ...p, metadata: { ...p.metadata, enabled } }
            : p
        ),
      }));
    } catch (error) {
      console.error("Failed to toggle plugin:", error);
      set({
        error: error instanceof Error ? error.message : "Failed to toggle plugin",
      });
      throw error;
    }
  },

  // 测试插件
  testPlugin: async (script: string, eventId: string) => {
    set({ testing: true, testResult: null, error: null });
    try {
      const result = await TestPlugin(script, eventId);
      const testResult = result as any;
      set({ testResult, testing: false });
      return testResult;
    } catch (error) {
      console.error("Failed to test plugin:", error);
      set({
        error: error instanceof Error ? error.message : "Failed to test plugin",
        testing: false,
      });
      throw error;
    }
  },

  // UI 操作
  openEditor: (plugin?: Plugin, initialTab: string = "basic") => {
    set({
      editorOpen: true,
      currentPlugin: plugin || null,
      editorInitialTab: initialTab,
      error: null,
    });
  },

  closeEditor: () => {
    set({
      editorOpen: false,
      currentPlugin: null,
      editorInitialTab: "basic", // 重置为默认
      error: null,
    });
  },

  setCurrentPlugin: (plugin: Plugin | null) => {
    set({ currentPlugin: plugin });
  },
}));
