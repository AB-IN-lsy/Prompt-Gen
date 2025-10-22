/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 23:33:13
 * @FilePath: \electron-go-app\frontend\src\pages\Settings.tsx
 * @LastEditTime: 2025-10-09 23:33:18
 */
import {
  ChangeEvent,
  FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { GlassCard } from "../components/ui/glass-card";
import { useAppSettings } from "../hooks/useAppSettings";
import { LANGUAGE_OPTIONS } from "../i18n";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Textarea } from "../components/ui/textarea";
import { AvatarUploader } from "../components/account/AvatarUploader";
import { useAuth } from "../hooks/useAuth";
import { PageHeader } from "../components/layout/PageHeader";
import { Download, LoaderCircle, Upload } from "lucide-react";
import {
  updateCurrentUser,
  requestEmailVerification,
  fetchUserModels,
  createUserModel,
  updateUserModel,
  deleteUserModel,
  fetchCurrentUser,
  testUserModel,
  exportPrompts,
  importPrompts,
  type UpdateCurrentUserRequest,
  type UserModelCredential,
  type CreateUserModelRequest,
  type UpdateUserModelRequest,
  type TestUserModelRequest,
  type ChatCompletionResponse,
  type PromptExportResult,
  type PromptImportResult,
} from "../lib/api";
import { ApiError, isApiError } from "../lib/errors";
import { EMAIL_VERIFIED_EVENT_KEY } from "../lib/verification";

type VerificationFeedback = {
  tone: "info" | "success" | "error";
  message: string;
  remaining?: number;
  retryAfter?: number;
};

type SettingsTab = "profile" | "models" | "app";

const TAB_QUERY_KEY = "tab";

const resolveTabFromParam = (value: string | null): SettingsTab => {
  switch ((value ?? "").toLowerCase()) {
    case "models":
      return "models";
    case "app":
      return "app";
    case "profile":
      return "profile";
    default:
      return "profile";
  }
};

export default function SettingsPage() {
  const { t } = useTranslation();
  const { language, setLanguage, theme, resolvedTheme, setTheme } =
    useAppSettings();
  const profile = useAuth((state) => state.profile);
  const setProfile = useAuth((state) => state.setProfile);
  const initializeAuth = useAuth((state) => state.initialize);
  const isEmailVerified = Boolean(profile?.user.email_verified_at);
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [importMode, setImportMode] = useState<"merge" | "overwrite">("merge");
  const [lastExport, setLastExport] = useState<PromptExportResult | null>(null);
  const [lastImport, setLastImport] = useState<PromptImportResult | null>(null);
  const formatDateTime = useCallback(
    (value?: string | null) => {
      if (!value) {
        return null;
      }
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) {
        return {
          date: value,
          time: "",
        };
      }
      const dateFormatter = new Intl.DateTimeFormat(language, {
        dateStyle: "short",
      });
      const timeFormatter = new Intl.DateTimeFormat(language, {
        timeStyle: "short",
      });
      return {
        date: dateFormatter.format(date),
        time: timeFormatter.format(date),
      };
    },
    [language],
  );
  const exportMutation = useMutation<PromptExportResult>({
    mutationFn: exportPrompts,
    onMutate: () => {
      toast.dismiss("settings-export");
      toast.loading(t("settings.backupCard.exportLoading"), {
        id: "settings-export",
      });
    },
    onSuccess: (result) => {
      toast.dismiss("settings-export");
      setLastExport(result);
      const key =
        result.promptCount > 0
          ? "settings.backupCard.exportSuccess"
          : "settings.backupCard.exportEmpty";
      toast.success(t(key, { count: result.promptCount, path: result.filePath }));
    },
    onError: (error: unknown) => {
      toast.dismiss("settings-export");
      const message =
        error instanceof Error
          ? error.message
          : t("settings.backupCard.exportFailed");
      toast.error(message);
    },
  });

  const importMutation = useMutation<
    PromptImportResult,
    unknown,
    { file: File; mode: "merge" | "overwrite" }
  >({
    mutationFn: ({ file, mode }) => importPrompts(file, mode),
    onMutate: () => {
      toast.dismiss("settings-import");
      toast.loading(t("settings.backupCard.importLoading"), {
        id: "settings-import",
      });
    },
    onSuccess: (result) => {
      toast.dismiss("settings-import");
      setLastImport(result);
      const successKey =
        result.errors.length > 0
          ? "settings.backupCard.importPartial"
          : "settings.backupCard.importSuccess";
      toast.success(
        t(successKey, {
          count: result.importedCount,
          skipped: result.skippedCount,
          errorCount: result.errors.length,
        }),
      );
      if (result.errors.length > 0) {
        const detail = result.errors
          .slice(0, 3)
          .map((item) => `${item.topic || "-"}: ${item.reason}`)
          .join("\n");
        if (detail) {
          toast.message(t("settings.backupCard.importErrorHint"), {
            description: detail,
          });
        }
      }
      void queryClient.invalidateQueries({ queryKey: ["my-prompts"] });
    },
    onError: (error: unknown) => {
      toast.dismiss("settings-import");
      const message =
        error instanceof Error
          ? error.message
          : t("settings.backupCard.importFailed");
      toast.error(message);
    },
  });

  const handleImportButtonClick = () => {
    fileInputRef.current?.click();
  };

  const handleImportFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    importMutation.mutate({ file, mode: importMode });
    event.target.value = "";
  };
  const exportTimestamp = useMemo(
    () => formatDateTime(lastExport?.generatedAt),
    [formatDateTime, lastExport],
  );
  const latestImportErrors = useMemo(() => {
    if (!lastImport) {
      return [];
    }
    return lastImport.errors.slice(0, 5);
  }, [lastImport]);
  const remainingImportErrors =
    lastImport && lastImport.errors.length > latestImportErrors.length
      ? lastImport.errors.length - latestImportErrors.length
      : 0;

  // 模型创建表单临时状态（界面提交时再转换）
  const [modelForm, setModelForm] = useState({
    provider: "deepseek",
    model_key: "",
    display_name: "",
    base_url: "",
    api_key: "",
    extra_config: "",
  });
  const [modelFormError, setModelFormError] = useState<string | null>(null);

  const [editModelForm, setEditModelForm] = useState({
    display_name: "",
    base_url: "",
    api_key: "",
    extra_config: "",
  });
  const [editingModelId, setEditingModelId] = useState<number | null>(null);
  const [editModelError, setEditModelError] = useState<string | null>(null);

  // 读取模型凭据列表，用于展示和联动
  const modelsQuery = useQuery<UserModelCredential[]>({
    queryKey: ["models"],
    queryFn: fetchUserModels,
  });

  // 成功操作后刷新用户资料，保证偏好模型实时同步
  const refreshProfile = useCallback(async () => {
    try {
      const next = await fetchCurrentUser();
      setProfile(next);
      return next;
    } catch (error) {
      console.error("Failed to refresh profile", error);
      return null;
    }
  }, [setProfile]);

 const [profileForm, setProfileForm] = useState({
    username: profile?.user.username ?? "",
    email: profile?.user.email ?? "",
    avatar_url: profile?.user.avatar_url ?? "",
    preferred_model: profile?.settings.preferred_model ?? "",
    enable_animations: profile?.settings.enable_animations ?? true,
  });

  const [profileErrors, setProfileErrors] = useState<{
    username?: string;
    email?: string;
  }>(() => ({
    username: undefined,
    email: undefined,
  }));

  const [verificationTargetEmail, setVerificationTargetEmail] = useState(
    profile?.user.email ?? "",
  );
  const [verificationFeedback, setVerificationFeedback] =
    useState<VerificationFeedback | null>(null);
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeTab, setActiveTab] = useState<SettingsTab>(() =>
    resolveTabFromParam(searchParams.get(TAB_QUERY_KEY)),
  );
  const tabParam = searchParams.get(TAB_QUERY_KEY);
  const tabItems = useMemo(
    () => [
      { id: "profile" as SettingsTab, label: t("settings.tabs.profile") },
      { id: "models" as SettingsTab, label: t("settings.tabs.models") },
      { id: "app" as SettingsTab, label: t("settings.tabs.app") },
    ],
    [t],
  );

  useEffect(() => {
    setProfileForm({
      username: profile?.user.username ?? "",
      email: profile?.user.email ?? "",
      avatar_url: profile?.user.avatar_url ?? "",
      preferred_model: profile?.settings.preferred_model ?? "",
      enable_animations: profile?.settings.enable_animations ?? true,
    });
    setProfileErrors({ username: undefined, email: undefined });
    setVerificationTargetEmail(profile?.user.email ?? "");
    setVerificationFeedback(null);
  }, [profile]);

  useEffect(() => {
    const paramTab = resolveTabFromParam(tabParam);
    if (paramTab !== activeTab) {
      setActiveTab(paramTab);
    }
  }, [activeTab, tabParam]);

  const handleTabChange = useCallback(
    (next: SettingsTab) => {
      const nextParams = new URLSearchParams(searchParams);
      if (next === "profile") {
        nextParams.delete(TAB_QUERY_KEY);
      } else {
        nextParams.set(TAB_QUERY_KEY, next);
      }
      setSearchParams(nextParams, { replace: true });
      setActiveTab(next);
    },
    [searchParams, setSearchParams],
  );

  const handleModelRequestError = useCallback(
    (error: unknown) => {
      if (error instanceof ApiError) {
        toast.error(error.message ?? t("errors.generic"));
      } else {
        toast.error(t("errors.generic"));
      }
    },
    [t],
  );

  const resetEditModelForm = useCallback(() => {
    setEditModelForm({
      display_name: "",
      base_url: "",
      api_key: "",
      extra_config: "",
    });
    setEditModelError(null);
    setEditingModelId(null);
  }, []);

  const formatVerifiedAt = useCallback(
    (value?: string | null) => {
      if (!value) {
        return t("settings.modelCard.lastVerifiedNever");
      }
      try {
        const date = new Date(value);
        if (Number.isNaN(date.getTime())) {
          return t("settings.modelCard.lastVerified", { time: value });
        }
        return t("settings.modelCard.lastVerified", {
          time: new Intl.DateTimeFormat(language, {
            dateStyle: "medium",
            timeStyle: "short",
          }).format(date),
        });
      } catch (error) {
        console.warn("Failed to format last verified timestamp", error);
        return t("settings.modelCard.lastVerified", { time: value });
      }
    },
    [language, t],
  );

  const openModelEditor = useCallback((credential: UserModelCredential) => {
    setEditingModelId(credential.id);
    setEditModelError(null);
    setEditModelForm({
      display_name: credential.display_name ?? "",
      base_url: credential.base_url ?? "",
      api_key: "",
      extra_config:
        credential.extra_config &&
        Object.keys(credential.extra_config).length > 0
          ? JSON.stringify(credential.extra_config, null, 2)
          : "",
    });
  }, []);

  const handleEditModelInputChange = useCallback(
    (field: keyof typeof editModelForm) =>
      (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        setEditModelError(null);
        const value = event.target.value;
        setEditModelForm((prev) => ({ ...prev, [field]: value }));
      },
    [],
  );

  const handleCancelEditModel = useCallback(() => {
    resetEditModelForm();
  }, [resetEditModelForm]);

  const createModelMutation = useMutation<
    UserModelCredential,
    unknown,
    CreateUserModelRequest
  >({
    mutationFn: (payload: CreateUserModelRequest) => createUserModel(payload),
    onSuccess: async () => {
      toast.success(t("settings.modelCard.createSuccess"));
      setModelForm({
        provider: "deepseek",
        model_key: "",
        display_name: "",
        base_url: "",
        api_key: "",
        extra_config: "",
      });
      setModelFormError(null);
      await queryClient.invalidateQueries({ queryKey: ["models"] });
    },
    onError: handleModelRequestError,
  });

  const updateModelMutation = useMutation<
    UserModelCredential,
    unknown,
    { id: number; data: UpdateUserModelRequest }
  >({
    mutationFn: ({ id, data }: { id: number; data: UpdateUserModelRequest }) =>
      updateUserModel(id, data),
    onSuccess: async (_credential, variables) => {
      await queryClient.invalidateQueries({ queryKey: ["models"] });
      if (variables.data.status) {
        const isDisabled = variables.data.status.toLowerCase() === "disabled";
        toast.success(
          isDisabled
            ? t("settings.modelCard.disableSuccess")
            : t("settings.modelCard.enableSuccess"),
        );
        if (isDisabled) {
          await refreshProfile();
        }
      } else {
        toast.success(t("settings.modelCard.updateSuccess"));
      }
    },
    onError: handleModelRequestError,
  });

  const handleSubmitEditModel = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (editingModelId === null) {
        return;
      }
      if (updateModelMutation.isPending) {
        return;
      }

      setEditModelError(null);

      const displayName = editModelForm.display_name.trim();
      const baseUrl = editModelForm.base_url.trim();
      const apiKey = editModelForm.api_key.trim();
      const extraRaw = editModelForm.extra_config.trim();

      if (!displayName) {
        setEditModelError(t("settings.modelCard.editDisplayNameRequired"));
        return;
      }

      let extraConfig: Record<string, unknown> = {};
      if (extraRaw) {
        try {
          const parsed = JSON.parse(extraRaw);
          if (parsed && typeof parsed === "object") {
            extraConfig = parsed as Record<string, unknown>;
          } else {
            throw new Error("invalid extra config");
          }
        } catch (error) {
          console.warn("Failed to parse edit extra_config", error);
          setEditModelError(t("settings.modelCard.extraConfigInvalid"));
          return;
        }
      }

      const payload: UpdateUserModelRequest = {
        display_name: displayName,
        base_url: baseUrl,
        extra_config: extraConfig,
      };
      if (apiKey) {
        payload.api_key = apiKey;
      }

      try {
        await updateModelMutation.mutateAsync({
          id: editingModelId,
          data: payload,
        });
        resetEditModelForm();
      } catch (error) {
        if (isApiError(error) && error.message) {
          setEditModelError(error.message);
        }
      }
    },
    [editModelForm, editingModelId, resetEditModelForm, t, updateModelMutation],
  );

  const deleteModelMutation = useMutation<void, unknown, number>({
    mutationFn: (id: number) => deleteUserModel(id),
    onSuccess: async (_, id) => {
      toast.success(t("settings.modelCard.deleteSuccess"));
      await queryClient.invalidateQueries({ queryKey: ["models"] });
      await refreshProfile();
    },
    onError: handleModelRequestError,
  });

  const setPreferredMutation = useMutation<
    Awaited<ReturnType<typeof updateCurrentUser>>,
    unknown,
    string
  >({
    mutationFn: (modelKey: string) =>
      updateCurrentUser({ preferred_model: modelKey }),
    onSuccess: (data) => {
      setProfile(data);
      toast.success(t("settings.modelCard.preferredSuccess"));
    },
    onError: handleModelRequestError,
  });

  const testModelMutation = useMutation<
    ChatCompletionResponse,
    unknown,
    { id: number; payload?: TestUserModelRequest }
  >({
    mutationFn: ({ id, payload }) => testUserModel(id, payload ?? {}),
    onSuccess: async (data) => {
      const reply = data?.choices?.[0]?.message?.content?.trim();
      if (reply) {
        const snippet = reply.length > 120 ? `${reply.slice(0, 117)}…` : reply;
        toast.success(
          t("settings.modelCard.testSuccessWithReply", { reply: snippet }),
        );
      } else {
        toast.success(t("settings.modelCard.testSuccess"));
      }
      await queryClient.invalidateQueries({ queryKey: ["models"] });
    },
    onError: handleModelRequestError,
  });

  useEffect(() => {
    if (typeof window === "undefined") {
      return undefined;
    }
    const syncProfile = () => {
      toast.success(t("settings.emailStatus.verifiedToast", "邮箱验证已完成"));
      void initializeAuth();
    };

    const checkLocalFlag = () => {
      const flag = window.localStorage.getItem(EMAIL_VERIFIED_EVENT_KEY);
      if (flag) {
        window.localStorage.removeItem(EMAIL_VERIFIED_EVENT_KEY);
        syncProfile();
        return true;
      }
      return false;
    };

    checkLocalFlag();

    const handleStorage = (event: StorageEvent) => {
      if (event.key === EMAIL_VERIFIED_EVENT_KEY && event.newValue) {
        window.localStorage.removeItem(EMAIL_VERIFIED_EVENT_KEY);
        syncProfile();
      }
    };

    window.addEventListener("storage", handleStorage);
    return () => {
      window.removeEventListener("storage", handleStorage);
    };
  }, [initializeAuth, t]);

  const mutation = useMutation({
    mutationFn: async (payload: UpdateCurrentUserRequest) =>
      updateCurrentUser(payload),
    onSuccess: (data) => {
      setProfile(data);
      toast.success(t("settings.profileSaveSuccess"));
    },
    onError: (error) => {
      if (error instanceof ApiError && error.code === "CONFLICT") {
        const details =
          (error.details as
            | { field?: string; fields?: string[] }
            | undefined) ?? {};
        const conflictFields = new Set<string>();
        if (typeof details.field === "string") {
          conflictFields.add(details.field);
        }
        if (Array.isArray(details.fields)) {
          for (const field of details.fields) {
            if (typeof field === "string") {
              conflictFields.add(field);
            }
          }
        }
        setProfileErrors((prev) => ({
          ...prev,
          email: conflictFields.has("email")
            ? t("settings.errors.emailTaken")
            : prev.email,
          username: conflictFields.has("username")
            ? t("settings.errors.usernameTaken")
            : prev.username,
        }));
      }
      const message =
        error instanceof ApiError
          ? (error.message ?? t("errors.generic"))
          : t("errors.generic");
      toast.error(message);
    },
  });

  const handleLanguageChange = (event: ChangeEvent<HTMLSelectElement>) => {
    const value = event.target.value;
    if (LANGUAGE_OPTIONS.some((option) => option.code === value)) {
      setLanguage(value as (typeof LANGUAGE_OPTIONS)[number]["code"]);
    }
  };

  const validateProfile = () => {
    const username = profileForm.username.trim();
    const email = profileForm.email.trim();
    const nextErrors = {
      username: username ? undefined : t("settings.errors.usernameRequired"),
      email: /^[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}$/.test(email)
        ? undefined
        : t("settings.errors.emailInvalid"),
    };
    setProfileErrors(nextErrors);
    return nextErrors;
  };

  const handleProfileSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (mutation.isPending) {
      return;
    }
    const errors = validateProfile();
    if (Object.values(errors).some(Boolean)) {
      return;
    }

    const payload: UpdateCurrentUserRequest = {
      username: profileForm.username.trim(),
      email: profileForm.email.trim(),
      preferred_model: profileForm.preferred_model.trim() || undefined,
      enable_animations: profileForm.enable_animations,
    };

    const initialAvatar = profile?.user.avatar_url ?? "";
    if (profileForm.avatar_url !== initialAvatar) {
      // 置空头像时需要显式发送空字符串，后端会将其写入数据库实现“移除头像”效果。
      const avatarValue = profileForm.avatar_url?.trim?.() ?? "";
      payload.avatar_url = avatarValue;
    }

    mutation.mutate(payload);
  };

  const animationLabel = useMemo(
    () =>
      profileForm.enable_animations
        ? t("settings.animationsEnabledOn")
        : t("settings.animationsEnabledOff"),
    [profileForm.enable_animations, t],
  );

  const handleToggleAnimations = useCallback(() => {
    const previousValue = profileForm.enable_animations;
    const nextValue = !previousValue;
    setProfileForm((prev) => ({ ...prev, enable_animations: nextValue }));
    mutation.mutate(
      { enable_animations: nextValue },
      {
        onError: () => {
          setProfileForm((prev) => ({ ...prev, enable_animations: previousValue }));
        },
      },
    );
  }, [mutation, profileForm.enable_animations]);

  const verificationMutation = useMutation({
    mutationFn: async () => {
      const email = profileForm.email.trim();
      if (!email) {
        throw new ApiError({
          code: "BAD_REQUEST",
          message: t("settings.verificationPending.emailMissing"),
        });
      }
      return requestEmailVerification(email);
    },
    onSuccess: (result) => {
      const email = profileForm.email.trim();
      setVerificationTargetEmail(email);
      const remaining =
        typeof result.remainingAttempts === "number"
          ? result.remainingAttempts
          : undefined;
      if (result.issued) {
        setVerificationFeedback({
          tone: "success",
          message: result.token
            ? t("settings.verificationPending.sentDev", { token: result.token })
            : t("settings.verificationPending.sent"),
          remaining,
        });
      } else {
        setVerificationFeedback({
          tone: "info",
          message: t("settings.verificationPending.sentNeutral"),
          remaining,
        });
      }
    },
    onError: (error) => {
      let message = t("errors.generic");
      let retryAfter: number | undefined;
      let remaining: number | undefined;
      if (isApiError(error)) {
        message = error.message ?? t("errors.generic");
        const details = (error.details ?? {}) as {
          remaining_attempts?: number;
          retry_after_seconds?: number;
        };
        if (typeof details.remaining_attempts === "number") {
          remaining = details.remaining_attempts;
        }
        if (typeof details.retry_after_seconds === "number") {
          retryAfter = details.retry_after_seconds;
        }
      } else if (error instanceof Error) {
        message = error.message;
      }

      setVerificationFeedback({
        tone: "error",
        message,
        remaining,
        retryAfter,
      });
    },
  });

  const handleModelInputChange =
    (field: keyof typeof modelForm) =>
    (
      event: ChangeEvent<
        HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement
      >,
    ) => {
      const rawValue = event.target.value;
      const value = field === "provider" ? rawValue.toLowerCase() : rawValue;
      setModelFormError(null);
      setModelForm((prev) => ({ ...prev, [field]: value }));
    };

  // 根据当前 provider 预设常见默认值，减少手动填写成本。
  const providerPreset = useMemo(() => {
    const normalized = (modelForm.provider || "deepseek").toLowerCase();
    if (normalized === "volcengine") {
      return {
        modelKey: "doubao-1-5-thinking-pro-250415",
        displayName: "Doubao",
        baseUrl: "https://ark.cn-beijing.volces.com/api/v3",
        extraConfig: '{"max_tokens":4096,"temperature":1}',
      };
    }
    return {
      modelKey: "deepseek-chat",
      displayName: "DeepSeek Chat",
      baseUrl: "https://api.deepseek.com/v1",
      extraConfig: '{"max_tokens":4096,"temperature":1}',
    };
  }, [modelForm.provider]);

  // 火山引擎默认要求特定 Base URL，自动补齐以避免请求失败。
  useEffect(() => {
    const normalized = (modelForm.provider || "deepseek").toLowerCase();
    if (normalized === "volcengine" && modelForm.base_url.trim() === "") {
      setModelForm((prev) => ({
        ...prev,
        base_url: providerPreset.baseUrl,
      }));
    }
  }, [modelForm.provider, modelForm.base_url, providerPreset.baseUrl]);

  // 表单校验 + 调用创建接口
  const handleCreateModelSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (createModelMutation.isPending) {
      return;
    }

    const provider = modelForm.provider.trim().toLowerCase();
    const modelKey = modelForm.model_key.trim();
    const displayName = modelForm.display_name.trim();
    const apiKey = modelForm.api_key.trim();
    const baseUrl = modelForm.base_url.trim();
    const extraRaw = modelForm.extra_config.trim();

    if (!provider || !modelKey || !displayName || !apiKey) {
      setModelFormError(t("settings.modelCard.formRequired"));
      return;
    }

    let extraConfig: Record<string, unknown> | undefined;
    if (extraRaw) {
      try {
        const parsed = JSON.parse(extraRaw);
        if (parsed && typeof parsed === "object") {
          extraConfig = parsed as Record<string, unknown>;
        } else {
          throw new Error("Invalid extra config");
        }
      } catch (error) {
        console.warn("Failed to parse extra_config", error);
        setModelFormError(t("settings.modelCard.extraConfigInvalid"));
        return;
      }
    }

    const payload: CreateUserModelRequest = {
      provider,
      model_key: modelKey,
      display_name: displayName,
      api_key: apiKey,
      base_url: baseUrl || undefined,
      extra_config: extraConfig,
    };

    createModelMutation.mutate(payload);
  };

  // 启用/禁用模型开关
  const handleToggleModelStatus = (credential: UserModelCredential) => {
    const currentStatus = (credential.status ?? "enabled").toLowerCase();
    const nextStatus = currentStatus === "enabled" ? "disabled" : "enabled";
    updateModelMutation.mutate({
      id: credential.id,
      data: { status: nextStatus },
    });
  };

  // 设置偏好模型（联动后端校验）
  const handleSetPreferredModel = (credential: UserModelCredential) => {
    if (
      setPreferredMutation.isPending &&
      setPreferredMutation.variables === credential.model_key
    ) {
      return;
    }
    setPreferredMutation.mutate(credential.model_key);
  };

  // 删除模型凭据，附带二次确认
  const handleDeleteModel = (credential: UserModelCredential) => {
    if (
      deleteModelMutation.isPending &&
      deleteModelMutation.variables !== undefined &&
      deleteModelMutation.variables === credential.id
    ) {
      return;
    }
    const confirmed = window.confirm(
      t("settings.modelCard.deleteConfirm", {
        name: credential.display_name || credential.model_key,
      }),
    );
    if (!confirmed) {
      return;
    }
    deleteModelMutation.mutate(credential.id);
  };

  const handleStartEditModel = (credential: UserModelCredential) => {
    openModelEditor(credential);
  };

  const handleTestModel = (credential: UserModelCredential) => {
    const provider = (credential.provider ?? "").toLowerCase();
    if (provider !== "deepseek" && provider !== "volcengine") {
      return;
    }
    if (
      testModelMutation.isPending &&
      testModelMutation.variables?.id === credential.id
    ) {
      return;
    }
    testModelMutation.mutate({ id: credential.id });
  };

  const themeOptions = useMemo(
    () => [
      {
        value: "system" as const,
        label: t("settings.themeCard.options.system"),
        description: t(
          "settings.themeCard.optionDescriptions.system",
          "跟随操作系统的外观设定",
        ),
      },
      {
        value: "light" as const,
        label: t("settings.themeCard.options.light"),
        description: t(
          "settings.themeCard.optionDescriptions.light",
          "使用浅色背景",
        ),
      },
      {
        value: "dark" as const,
        label: t("settings.themeCard.options.dark"),
        description: t(
          "settings.themeCard.optionDescriptions.dark",
          "使用深色背景",
        ),
      },
    ],
    [t],
  );

  const models = modelsQuery.data ?? [];
  const currentPreferred = profile?.settings?.preferred_model ?? "";

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 text-slate-700 transition-colors dark:text-slate-200">
      <PageHeader
        eyebrow={t("settings.eyebrow")}
        title={t("settings.title")}
        description={t("settings.subtitle")}
      />

      <div className="flex overflow-x-auto rounded-2xl border border-white/60 bg-white/70 p-1 text-sm shadow-sm dark:border-slate-800 dark:bg-slate-900/60">
        {tabItems.map((item) => {
          const isActive = activeTab === item.id;
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => handleTabChange(item.id)}
              className={`relative min-w-[120px] flex-1 whitespace-nowrap rounded-xl px-4 py-2 font-medium transition ${
                isActive
                  ? "bg-primary text-white shadow-glow"
                  : "text-slate-600 hover:bg-white/80 dark:text-slate-300 dark:hover:bg-slate-800/70"
              }`}
            >
              {item.label}
            </button>
          );
        })}
      </div>

      {activeTab === "profile" ? (
        <>
          {isEmailVerified ? null : (
            <div className="rounded-xl border border-amber-200 bg-amber-50/80 p-4 text-sm leading-relaxed text-amber-800 transition-colors dark:border-amber-400/50 dark:bg-amber-500/10 dark:text-amber-100">
              <p>
                {t(
                  "settings.verificationPending.notice",
                  "邮箱尚未完成验证，请尽快前往验证以解锁全部功能。",
                )}
              </p>
              {verificationTargetEmail ? (
                <p className="mt-2 text-xs text-amber-700 dark:text-amber-200">
                  {t("settings.verificationPending.emailLabel", {
                    email: verificationTargetEmail,
                  })}
                </p>
              ) : null}
              <div className="mt-3 flex flex-wrap gap-2">
                <Button
                  type="button"
                  disabled={verificationMutation.isPending}
                  onClick={() => verificationMutation.mutate()}
                >
                  {verificationMutation.isPending
                    ? t("common.loading")
                    : t("settings.verificationPending.send", "发送验证邮件")}
                </Button>
              </div>
              {verificationFeedback ? (
                <div
                  className={`mt-3 rounded-lg border px-3 py-2 text-xs transition-colors ${
                    verificationFeedback.tone === "success"
                      ? "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-400/40 dark:bg-emerald-500/10 dark:text-emerald-200"
                      : verificationFeedback.tone === "error"
                        ? "border-red-200 bg-red-50 text-red-700 dark:border-red-400/50 dark:bg-red-500/10 dark:text-red-200"
                        : "border-slate-200 bg-white/70 text-slate-600 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-200"
                  }`}
                >
                  <p>{verificationFeedback.message}</p>
                  {typeof verificationFeedback.remaining === "number" ? (
                    <p className="mt-1">
                      {t("settings.verificationPending.remaining", {
                        count: verificationFeedback.remaining,
                      })}
                    </p>
                  ) : null}
                  {typeof verificationFeedback.retryAfter === "number" ? (
                    <p className="mt-1">
                      {t("settings.verificationPending.rateLimit", {
                        seconds: verificationFeedback.retryAfter,
                      })}
                    </p>
                  ) : null}
                </div>
              ) : null}
            </div>
          )}

          <GlassCard className="space-y-6">
            <form onSubmit={handleProfileSubmit} className="space-y-6">
          <div>
            <h2 className="text-lg font-medium text-slate-800 dark:text-slate-100">
              {t("settings.profileCard.title")}
            </h2>
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
              {t("settings.profileCard.description")}
            </p>
          </div>

          <AvatarUploader
            value={profileForm.avatar_url}
            onChange={(value) => {
              setProfileForm((prev) => ({ ...prev, avatar_url: value }));
            }}
          />

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <label
                className="text-sm font-medium text-slate-700 dark:text-slate-200"
                htmlFor="profile-username"
              >
                {t("settings.profileCard.username")}
              </label>
              <Input
                id="profile-username"
                value={profileForm.username}
                autoComplete="username"
                onChange={(event) => {
                  const value = event.target.value;
                  setProfileForm((prev) => ({ ...prev, username: value }));
                  setProfileErrors((prev) => ({
                    ...prev,
                    username: undefined,
                  }));
                }}
                required
              />
              {profileErrors.username ? (
                <p className="text-xs text-red-500">{profileErrors.username}</p>
              ) : null}
            </div>

            <div className="space-y-2">
              <label
                className="flex items-center gap-2 text-sm font-medium text-slate-700 dark:text-slate-200"
                htmlFor="profile-email"
              >
                {t("settings.profileCard.email")}
                {isEmailVerified ? (
                  <Badge className="bg-emerald-500/15 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-200">
                    {t("settings.emailStatus.verified", "已验证")}
                  </Badge>
                ) : (
                  <Badge
                    variant="outline"
                    className="border-amber-400/60 text-amber-700 dark:border-amber-300/50 dark:text-amber-200"
                  >
                    {t("settings.emailStatus.pending", "未验证")}
                  </Badge>
                )}
              </label>
              <Input
                id="profile-email"
                type="email"
                value={profileForm.email}
                autoComplete="email"
                onChange={(event) => {
                  const value = event.target.value;
                  setProfileForm((prev) => ({ ...prev, email: value }));
                  setProfileErrors((prev) => ({ ...prev, email: undefined }));
                }}
                required
              />
              {profileErrors.email ? (
                <p className="text-xs text-red-500">{profileErrors.email}</p>
              ) : null}
              {isEmailVerified ? null : (
                <>
                  <p className="mt-2 text-xs text-amber-700 dark:text-amber-200">
                    {verificationTargetEmail
                      ? t("settings.verificationPending.emailLabel", {
                          email: verificationTargetEmail,
                        })
                      : t("settings.verificationPending.emailLabelFallback")}
                  </p>
                </>
              )}
            </div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <label
                className="text-sm font-medium text-slate-700 dark:text-slate-200"
                htmlFor="profile-model"
              >
                {t("settings.profileCard.preferredModel")}
              </label>
              <Input
                id="profile-model"
                value={profileForm.preferred_model}
                onChange={(event) =>
                  setProfileForm((prev) => ({
                    ...prev,
                    preferred_model: event.target.value,
                  }))
                }
                placeholder={
                  t("settings.profileCard.preferredModelPlaceholder") ?? ""
                }
              />
            </div>
          </div>

          <div className="flex justify-end">
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t("common.loading")
                : t("settings.profileCard.save")}
            </Button>
          </div>
        </form>
      </GlassCard>
        </>
      ) : null}

      {activeTab === "models" ? (
      <GlassCard className="space-y-6">
        <div>
          <h2 className="text-lg font-medium text-slate-800 dark:text-slate-100">
            {t("settings.modelCard.title")}
          </h2>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            {t("settings.modelCard.description")}
          </p>
          <p className="mt-1 text-xs text-slate-400 dark:text-slate-500">
            {t("settings.modelCard.providersNote")}
          </p>
        </div>

        <div className="space-y-3">
          {modelsQuery.isPending ? (
            <p className="text-sm text-slate-500 dark:text-slate-400">
              {t("common.loading")}
            </p>
          ) : modelsQuery.isError ? (
            <div className="rounded-xl border border-red-200 bg-red-50/70 px-4 py-3 text-sm text-red-700 dark:border-red-400/40 dark:bg-red-500/10 dark:text-red-200">
              <p>{t("settings.modelCard.fetchError")}</p>
              <Button
                type="button"
                variant="ghost"
                className="mt-2 text-sm text-red-600 hover:text-red-700 dark:text-red-300 dark:hover:text-red-200"
                onClick={() => modelsQuery.refetch()}
              >
                {t("settings.modelCard.retry")}
              </Button>
            </div>
          ) : models.length === 0 ? (
            <div className="rounded-xl border border-dashed border-slate-200 bg-white/60 px-4 py-6 text-center text-sm text-slate-500 dark:border-slate-700 dark:bg-slate-900/60 dark:text-slate-400">
              {t("settings.modelCard.empty")}
            </div>
          ) : (
            // 已有凭据列表，逐条展示操作按钮
            models.map((credential) => {
              const isPreferred =
                currentPreferred && credential.model_key === currentPreferred;
              const isDisabled =
                String(credential.status ?? "").toLowerCase() === "disabled";
              const updateInFlight =
                updateModelMutation.isPending &&
                updateModelMutation.variables?.id === credential.id;
              const toggleDisabled = updateInFlight;
              const deleteDisabled =
                deleteModelMutation.isPending &&
                deleteModelMutation.variables === credential.id;
              const setPreferredDisabled =
                isDisabled ||
                isPreferred ||
                (setPreferredMutation.isPending &&
                  setPreferredMutation.variables === credential.model_key);
              const isEditing = editingModelId === credential.id;
              const editDisabled = isEditing || updateInFlight;
              const providerNormalized =
                credential.provider?.toLowerCase() ?? "";
              const isTestable =
                providerNormalized === "deepseek" ||
                providerNormalized === "volcengine";
              const testingDisabled =
                !isTestable ||
                (testModelMutation.isPending &&
                  testModelMutation.variables?.id === credential.id);

              return (
                <div
                  key={credential.id}
                  className="rounded-xl border border-white/60 bg-white/70 px-4 py-3 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70"
                >
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div className="space-y-1 sm:flex-1 sm:min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-sm font-medium text-slate-800 dark:text-slate-100">
                          {credential.display_name || credential.model_key}
                        </span>
                        {isPreferred ? (
                          <Badge className="bg-primary/10 text-primary dark:bg-primary/20">
                            {t("settings.modelCard.preferredBadge")}
                          </Badge>
                        ) : null}
                        <Badge
                          variant={isDisabled ? "outline" : "default"}
                          className={
                            isDisabled
                              ? "bg-slate-200 text-slate-600 dark:bg-slate-800/70 dark:text-slate-300"
                              : ""
                          }
                        >
                          {isDisabled
                            ? t("settings.modelCard.statusDisabled")
                            : t("settings.modelCard.statusEnabled")}
                        </Badge>
                      </div>
                      <p className="text-xs text-slate-500 dark:text-slate-400 break-words">
                        {t("settings.modelCard.meta", {
                          provider: credential.provider,
                          model: credential.model_key,
                        })}
                      </p>
                      {credential.base_url ? (
                        <p className="text-xs text-slate-400 dark:text-slate-500 break-words">
                          {t("settings.modelCard.baseUrl", {
                            url: credential.base_url,
                          })}
                        </p>
                      ) : null}
                      <p className="text-xs text-slate-400 dark:text-slate-500">
                        {formatVerifiedAt(credential.last_verified_at)}
                      </p>
                    </div>
                    <div className="flex flex-wrap gap-2 sm:flex-none sm:items-center sm:justify-end">
                      <Button
                        type="button"
                        variant="outline"
                        disabled={testingDisabled}
                        onClick={() => handleTestModel(credential)}
                      >
                        {testModelMutation.isPending &&
                        testModelMutation.variables?.id === credential.id
                          ? t("settings.modelCard.testing")
                          : t("settings.modelCard.test")}
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        disabled={setPreferredDisabled}
                        onClick={() => handleSetPreferredModel(credential)}
                      >
                        {isPreferred
                          ? t("settings.modelCard.preferredCurrent")
                          : t("settings.modelCard.setPreferred")}
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        disabled={toggleDisabled}
                        onClick={() => handleToggleModelStatus(credential)}
                      >
                        {isDisabled
                          ? t("settings.modelCard.enable")
                          : t("settings.modelCard.disable")}
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        disabled={editDisabled}
                        onClick={() => handleStartEditModel(credential)}
                      >
                        {t("settings.modelCard.edit")}
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        className="text-red-600 hover:text-red-700 dark:text-red-300 dark:hover:text-red-200"
                        disabled={deleteDisabled}
                        onClick={() => handleDeleteModel(credential)}
                      >
                        {t("settings.modelCard.delete")}
                      </Button>
                    </div>
                  </div>
                  {isEditing ? (
                    <form
                      className="mt-3 space-y-3 rounded-xl border border-primary/20 bg-primary/5 p-3 dark:border-primary/30 dark:bg-primary/10"
                      onSubmit={handleSubmitEditModel}
                    >
                      <h4 className="text-sm font-semibold text-slate-700 dark:text-slate-200">
                        {t("settings.modelCard.editTitle", {
                          name: credential.display_name || credential.model_key,
                        })}
                      </h4>
                      <div className="grid gap-3 md:grid-cols-2">
                        <div className="flex flex-col gap-2">
                          <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                            {t("settings.modelCard.displayName")}
                          </label>
                          <Input
                            value={editModelForm.display_name}
                            onChange={handleEditModelInputChange(
                              "display_name",
                            )}
                          />
                        </div>
                        <div className="flex flex-col gap-2">
                          <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                            {t("settings.modelCard.baseUrlLabel")}
                          </label>
                          <Input
                            value={editModelForm.base_url}
                            onChange={handleEditModelInputChange("base_url")}
                            placeholder="https://api.example.com"
                          />
                        </div>
                        <div className="flex flex-col gap-2 md:col-span-2">
                          <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                            {t("settings.modelCard.apiKey")}
                          </label>
                          <Input
                            value={editModelForm.api_key}
                            onChange={handleEditModelInputChange("api_key")}
                            type="password"
                            placeholder="sk-..."
                          />
                          <span className="text-xs text-slate-400 dark:text-slate-500">
                            {t("settings.modelCard.editApiKeyHint")}
                          </span>
                        </div>
                        <div className="flex flex-col gap-2 md:col-span-2">
                          <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                            {t("settings.modelCard.extraConfig")}
                          </label>
                          <Textarea
                            value={editModelForm.extra_config}
                            onChange={handleEditModelInputChange(
                              "extra_config",
                            )}
                            rows={3}
                          />
                          <span className="text-xs text-slate-400 dark:text-slate-500">
                            {t("settings.modelCard.extraConfigHint")}
                          </span>
                        </div>
                      </div>
                      {editModelError ? (
                        <p className="rounded-lg border border-red-200 bg-red-50/70 px-3 py-2 text-xs text-red-700 dark:border-red-400/40 dark:bg-red-500/10 dark:text-red-200">
                          {editModelError}
                        </p>
                      ) : null}
                      <div className="flex justify-end gap-2">
                        <Button
                          type="button"
                          variant="ghost"
                          onClick={handleCancelEditModel}
                        >
                          {t("settings.modelCard.editCancel")}
                        </Button>
                        <Button
                          type="submit"
                          disabled={
                            updateModelMutation.isPending &&
                            updateModelMutation.variables?.id === credential.id
                          }
                        >
                          {updateModelMutation.isPending &&
                          updateModelMutation.variables?.id === credential.id
                            ? t("common.loading")
                            : t("settings.modelCard.editSave")}
                        </Button>
                      </div>
                    </form>
                  ) : null}
                </div>
              );
            })
          )}
        </div>

        <div className="border-t border-white/60 pt-4 dark:border-slate-800/70">
          {/* 新增模型表单，默认保存 JSON 字符串 */}
          <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200">
            {t("settings.modelCard.addTitle")}
          </h3>
          <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
            {t("settings.modelCard.addDescription")}
          </p>
          <form
            className="mt-4 grid gap-3 md:grid-cols-2"
            onSubmit={handleCreateModelSubmit}
          >
            <div className="flex flex-col gap-2">
              <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                {t("settings.modelCard.provider")}
              </label>
              <select
                value={modelForm.provider}
                onChange={handleModelInputChange("provider")}
                className="h-10 w-full rounded-xl border border-white/70 bg-white/80 px-3 text-sm text-slate-700 shadow-sm transition focus:border-primary focus:shadow-glow focus:outline-none dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200 dark:focus:border-primary/60"
              >
                <option value="deepseek">
                  {t("settings.modelCard.providerOptionDeepseek")}
                </option>
                <option value="volcengine">
                  {t("settings.modelCard.providerOptionVolcengine")}
                </option>
              </select>
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {t("settings.modelCard.providerHint")}
              </span>
            </div>
            <div className="flex flex-col gap-2">
              <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                {t("settings.modelCard.modelKey")}
              </label>
              <Input
                value={modelForm.model_key}
                onChange={handleModelInputChange("model_key")}
                placeholder={providerPreset.modelKey}
              />
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {t("settings.modelCard.modelKeyHint")}
              </span>
            </div>
            <div className="flex flex-col gap-2">
              <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                {t("settings.modelCard.displayName")}
              </label>
              <Input
                value={modelForm.display_name}
                onChange={handleModelInputChange("display_name")}
                placeholder={providerPreset.displayName}
              />
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {t("settings.modelCard.displayNameHint")}
              </span>
            </div>
            <div className="flex flex-col gap-2">
              <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                {t("settings.modelCard.baseUrlLabel")}
              </label>
              <Input
                value={modelForm.base_url}
                onChange={handleModelInputChange("base_url")}
                placeholder={providerPreset.baseUrl}
              />
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {t("settings.modelCard.baseUrlHint")}
              </span>
            </div>
            <div className="flex flex-col gap-2 md:col-span-2">
              <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                {t("settings.modelCard.apiKey")}
              </label>
              <Input
                value={modelForm.api_key}
                onChange={handleModelInputChange("api_key")}
                type="password"
                placeholder="sk-xxxxxxxx"
              />
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {t("settings.modelCard.apiKeyHint")}
              </span>
            </div>
            <div className="flex flex-col gap-2 md:col-span-2">
              <label className="text-xs font-medium text-slate-500 dark:text-slate-400">
                {t("settings.modelCard.extraConfig")}
              </label>
              <Textarea
                value={modelForm.extra_config}
                onChange={handleModelInputChange("extra_config")}
                placeholder={providerPreset.extraConfig}
                rows={3}
              />
              <span className="text-xs text-slate-400 dark:text-slate-500">
                {t("settings.modelCard.extraConfigHint")}
              </span>
            </div>
            {modelFormError ? (
              <div className="md:col-span-2">
                <p className="rounded-lg border border-red-200 bg-red-50/70 px-3 py-2 text-xs text-red-700 dark:border-red-400/40 dark:bg-red-500/10 dark:text-red-200">
                  {modelFormError}
                </p>
              </div>
            ) : null}
            <div className="md:col-span-2 flex justify-end">
              <Button type="submit" disabled={createModelMutation.isPending}>
                {createModelMutation.isPending
                  ? t("common.loading")
                  : t("settings.modelCard.createButton")}
              </Button>
            </div>
          </form>
        </div>
      </GlassCard>
      ) : null}

      {activeTab === "app" ? (
      <>
      <GlassCard className="space-y-4">
        <div>
          <h2 className="text-lg font-medium text-slate-800 dark:text-slate-100">
            {t("settings.animationCard.title")}
          </h2>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            {t("settings.animationCard.description")}
          </p>
        </div>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-1 text-sm text-slate-500 dark:text-slate-400">
            <p>{t("settings.animationCard.helper")}</p>
            <p className="text-xs text-slate-400 dark:text-slate-500">
              {t("settings.animationCard.note")}
            </p>
          </div>
          <button
            type="button"
            className="flex w-full max-w-sm items-center justify-between rounded-xl border border-white/60 bg-white/80 px-4 py-2 text-left text-sm text-slate-700 shadow-sm transition hover:border-primary dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200"
            onClick={handleToggleAnimations}
            disabled={mutation.isPending}
          >
            <span>{t("settings.animationCard.toggleLabel")}</span>
            <span className="text-xs text-primary">{animationLabel}</span>
          </button>
        </div>
      </GlassCard>
      <GlassCard className="space-y-4">
        <div>
          <h2 className="text-lg font-medium text-slate-800 dark:text-slate-100">
            {t("settings.backupCard.title")}
          </h2>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            {t("settings.backupCard.description")}
          </p>
        </div>
        <div className="space-y-4">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex items-center gap-2 rounded-full border border-white/60 bg-white/80 px-3 py-1 shadow-sm transition-colors dark:border-slate-800 dark:bg-slate-900/70">
              <label
                className="text-xs font-medium text-slate-500 dark:text-slate-400"
                htmlFor="backup-import-mode"
              >
                {t("settings.backupCard.importMode")}
              </label>
              <select
                id="backup-import-mode"
                className="rounded-md border border-transparent bg-white/90 px-2 py-1 text-xs text-slate-700 shadow-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/40 dark:border-slate-700 dark:bg-slate-900/80 dark:text-slate-200 dark:shadow-none"
                value={importMode}
                onChange={(event) =>
                  setImportMode(event.target.value as "merge" | "overwrite")
                }
                disabled={importMutation.isPending}
              >
                <option value="merge">{t("settings.backupCard.modeMerge")}</option>
                <option value="overwrite">
                  {t("settings.backupCard.modeOverwrite")}
                </option>
              </select>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={importMutation.isPending}
                onClick={handleImportButtonClick}
                className="shadow-sm dark:shadow-none"
              >
                {importMutation.isPending ? (
                  <>
                    <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                    {t("settings.backupCard.importLoading")}
                  </>
                ) : (
                  <>
                    <Upload className="mr-2 h-4 w-4" />
                    {t("settings.backupCard.importButton")}
                  </>
                )}
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={exportMutation.isPending}
                onClick={() => exportMutation.mutate()}
                className="shadow-sm dark:shadow-none"
              >
                {exportMutation.isPending ? (
                  <>
                    <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                    {t("settings.backupCard.exportLoading")}
                  </>
                ) : (
                  <>
                    <Download className="mr-2 h-4 w-4" />
                    {t("settings.backupCard.exportButton")}
                  </>
                )}
              </Button>
            </div>
          </div>
          <input
            ref={fileInputRef}
            type="file"
            accept="application/json"
            className="hidden"
            onChange={handleImportFileChange}
          />
          {lastExport ? (
            <div className="rounded-3xl border border-primary/20 bg-white/80 px-4 py-3 text-sm text-slate-600 shadow-sm transition-colors dark:border-primary/25 dark:bg-slate-900/70 dark:text-slate-200">
              <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                <div className="space-y-1">
                  <span className="text-xs font-semibold uppercase tracking-[0.26em] text-primary dark:text-primary/80">
                    {t("settings.backupCard.lastExport")}
                  </span>
                  <p className="font-mono text-xs text-slate-500 dark:text-slate-400">
                    {t("settings.backupCard.filePath")}：
                    <span className="ml-1 break-all text-slate-700 dark:text-slate-100">
                      {lastExport.filePath}
                    </span>
                  </p>
                  {exportTimestamp ? (
                    <p className="text-xs text-slate-500 dark:text-slate-400">
                      {t("settings.backupCard.time", {
                        date: exportTimestamp.date,
                        time: exportTimestamp.time,
                      })}
                    </p>
                  ) : null}
                </div>
                <Badge variant="outline" className="whitespace-nowrap text-xs">
                  {t("settings.backupCard.promptCount", {
                    count: lastExport.promptCount,
                  })}
                </Badge>
              </div>
            </div>
          ) : null}

          {lastImport ? (
            <div className="rounded-3xl border border-emerald-200/60 bg-emerald-50/70 px-4 py-3 text-sm text-emerald-700 shadow-sm transition-colors dark:border-emerald-400/40 dark:bg-emerald-500/10 dark:text-emerald-200">
              <div className="space-y-2">
                <span className="text-xs font-semibold uppercase tracking-[0.26em] text-emerald-500 dark:text-emerald-300">
                  {t("settings.backupCard.lastImport")}
                </span>
                <p className="text-xs">
                  {t("settings.backupCard.importSummary", {
                    count: lastImport.importedCount,
                    skipped: lastImport.skippedCount,
                  })}
                </p>
                {latestImportErrors.length > 0 ? (
                  <div className="rounded-2xl border border-amber-200/60 bg-amber-50/80 px-3 py-2 text-xs text-amber-700 dark:border-amber-400/40 dark:bg-amber-500/10 dark:text-amber-200">
                    <p className="font-medium">
                      {t("settings.backupCard.importErrorsHeading", {
                        count: lastImport.errors.length,
                      })}
                    </p>
                    <ul className="mt-1 list-disc space-y-1 pl-5">
                      {latestImportErrors.map((item, index) => (
                        <li key={`${item.topic}-${index}`} className="break-all">
                          <span className="font-semibold">
                            {item.topic || t("settings.backupCard.unknownTopic")}
                          </span>
                          <span className="ml-1 text-slate-600 dark:text-slate-200">
                            {item.reason}
                          </span>
                        </li>
                      ))}
                      {remainingImportErrors > 0 ? (
                        <li className="italic text-slate-500 dark:text-slate-300">
                          {t("settings.backupCard.moreErrors", {
                            remaining: remainingImportErrors,
                          })}
                        </li>
                      ) : null}
                    </ul>
                  </div>
                ) : null}
                <Badge variant="outline" className="w-max whitespace-nowrap text-xs">
                  {t("settings.backupCard.importResultBadge", {
                    count: lastImport.importedCount,
                  })}
                </Badge>
              </div>
            </div>
          ) : null}
        </div>
      </GlassCard>

      <GlassCard className="space-y-4">
        <div>
          <h2 className="text-lg font-medium text-slate-800 dark:text-slate-100">
            {t("settings.themeCard.title")}
          </h2>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            {t("settings.themeCard.description")}
          </p>
        </div>
        <div className="grid gap-3 sm:grid-cols-3">
          {themeOptions.map((option) => {
            const isActive = option.value === theme;
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => setTheme(option.value)}
                className={`rounded-xl border px-4 py-3 text-left transition ${
                  isActive
                    ? "border-primary bg-primary/10 text-primary"
                    : "border-white/60 bg-white/80 text-slate-700 hover:border-primary dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200"
                }`}
              >
                <span className="block text-sm font-medium dark:text-slate-100">
                  {option.label}
                </span>
                <span className="mt-1 block text-xs text-slate-500 dark:text-slate-400">
                  {option.value === "system"
                    ? t("settings.themeCard.systemHint", {
                        mode:
                          resolvedTheme === "dark"
                            ? t("settings.themeCard.systemHintDark", "深色")
                            : t("settings.themeCard.systemHintLight", "浅色"),
                      })
                    : option.description}
                </span>
              </button>
            );
          })}
        </div>
      </GlassCard>

      <GlassCard className="space-y-4">
        <div>
          <h2 className="text-lg font-medium text-slate-800 dark:text-slate-100">
            {t("settings.languageCardTitle")}
          </h2>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            {t("settings.languageCardDescription")}
          </p>
        </div>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <label
            className="text-sm font-medium text-slate-600 dark:text-slate-300"
            htmlFor="language-select"
          >
            {t("settings.languageSelectLabel")}
          </label>
          <select
            id="language-select"
            className="w-full rounded-xl border border-white/60 bg-white/80 px-3 py-2 text-sm text-slate-700 shadow-sm focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/40 sm:w-64 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200"
            value={language}
            onChange={handleLanguageChange}
          >
            {LANGUAGE_OPTIONS.map((option) => (
              <option key={option.code} value={option.code}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
      </GlassCard>
      </>
      ) : null}
    </div>
  );
}
