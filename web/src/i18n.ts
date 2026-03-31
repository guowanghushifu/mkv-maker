export type Locale = 'zh' | 'en';

export const localeStorageKey = 'mkv-maker-locale';
export const tokenStorageKey = 'mkv-maker-token';

type MessageSet = {
  layout: {
    appEyebrow: string;
    appTitle: string;
    appSubtitle: string;
    workflowStepsAria: string;
    localeToggle: string;
    contextTitle: string;
    contextLabels: {
      source: string;
      playlist: string;
      output: string;
      task: string;
    };
    readyState: string;
    lockedState: string;
    pendingValue: string;
    steps: Record<'login' | 'scan' | 'bdinfo' | 'editor' | 'review', string>;
  };
  status: {
    running: string;
    succeeded: string;
    failed: string;
  };
  login: {
    title: string;
    subtitle: string;
    passwordLabel: string;
    passwordPlaceholder: string;
    continueButton: string;
    passwordRequired: string;
  };
  scan: {
    title: string;
    subtitle: string;
    scanButton: string;
    scanningButton: string;
    nextButton: string;
    empty: string;
    columns: {
      select: string;
      name: string;
      type: string;
      path: string;
      size: string;
      modified: string;
    };
    typeBDMV: string;
    selectSource: (name: string) => string;
  };
  bdinfo: {
    title: string;
    selectedSource: string;
    description: string;
    placeholder: string;
    sampleTitle: string;
    playlist: string;
    audioLabelsFound: string;
    subtitleLabelsFound: string;
    backButton: string;
    submitButton: string;
    submittingButton: string;
  };
  editor: {
    title: string;
    titleLabel: string;
    titleHint: string;
    videoTrackNameLabel: string;
    videoSourceAttributes: string;
    liveFilenamePreview: string;
    outputFilename: string;
    audioHeading: string;
    subtitlesHeading: string;
    noSubtitles: string;
    backButton: string;
    nextButton: string;
    columns: {
      drag: string;
      id: string;
      track: string;
      language: string;
      audioFormat: string;
      include: string;
      default: string;
    };
    dragTrack: (name: string) => string;
    trackName: (name: string) => string;
    language: (name: string) => string;
    include: (name: string) => string;
    default: (name: string) => string;
  };
  review: {
    title: string;
    description: string;
    source: string;
    playlist: string;
    filename: string;
    outputPath: string;
    dolbyVisionMergeEnabled: string;
    yes: string;
    no: string;
    finalTrackList: string;
    video: string;
    audio: string;
    subtitle: string;
    defaultFlag: string;
    forcedFlag: string;
    backButton: string;
    startRemuxButton: string;
    startingRemuxButton: string;
    startNextRemuxButton: string;
    currentRemux: string;
    output: string;
    path: string;
    progress: string;
    commandPreview: string;
    logOutput: string;
    waitingForLogOutput: string;
  };
  app: {
    loginFailed: string;
    scanFailed: string;
    bdinfoParseFailed: string;
    currentJobRunning: string;
    submitFailed: string;
  };
};

export const messages: Record<Locale, MessageSet> = {
  zh: {
    layout: {
      appEyebrow: 'Media Mastering Console',
      appTitle: 'MKV Remux Tool',
      appSubtitle: '针对蓝光 Remux 工作流的精确控制台，串联 BDInfo、轨道整理与任务执行。',
      workflowStepsAria: '工作流步骤',
      localeToggle: '中文 / EN',
      contextTitle: '当前会话',
      contextLabels: {
        source: '片源',
        playlist: '播放列表',
        output: '输出',
        task: '任务',
      },
      readyState: '就绪',
      lockedState: '未登录',
      pendingValue: '等待上一步',
      steps: {
        login: '登录',
        scan: '扫描',
        bdinfo: 'BDInfo提取',
        editor: '轨道编辑',
        review: '任务概览',
      },
    },
    status: {
      running: '进行中',
      succeeded: '已完成',
      failed: '失败',
    },
    login: {
      title: '登录',
      subtitle: '单用户本地访问。',
      passwordLabel: '密码',
      passwordPlaceholder: '请输入密码',
      continueButton: '继续',
      passwordRequired: '请输入密码。',
    },
    scan: {
      title: '扫描片源',
      subtitle: '仅支持已解压的 BDMV 目录作为工作流输入。',
      scanButton: '扫描片源',
      scanningButton: '扫描中...',
      nextButton: '下一步',
      empty: '暂无片源，请先扫描。',
      columns: {
        select: '选择',
        name: '名称',
        type: '类型',
        path: '路径',
        size: '大小',
        modified: '修改时间',
      },
      typeBDMV: 'BDMV 目录',
      selectSource: (name: string) => `选择 ${name}`,
    },
    bdinfo: {
      title: 'BDInfo信息',
      selectedSource: '已选片源',
      description: '请粘贴 BDInfo 文本。此步骤必填，无法跳过。',
      placeholder: '请在这里粘贴完整 BDInfo 文本',
      sampleTitle: 'BDInfo 样例',
      playlist: '播放列表',
      audioLabelsFound: '检测到音频标签',
      subtitleLabelsFound: '检测到字幕标签',
      backButton: '返回',
      submitButton: '下一步',
      submittingButton: '解析中...',
    },
    editor: {
      title: '轨道编辑器',
      titleLabel: '标题',
      titleHint: '建议名称+年份，示例：Nightcrawler.2014',
      videoTrackNameLabel: '视频轨道名称',
      videoSourceAttributes: '视频源属性',
      liveFilenamePreview: '实时文件名预览',
      outputFilename: '输出文件名',
      audioHeading: '音频',
      subtitlesHeading: '字幕',
      noSubtitles: '此草稿中未发现字幕。',
      backButton: '返回',
      nextButton: '下一步',
      columns: {
        drag: '移动',
        id: 'ID',
        track: '轨道',
        language: '语言',
        audioFormat: '音轨格式',
        include: '保留',
        default: '默认',
      },
      dragTrack: (name: string) => `拖拽 ${name}`,
      trackName: (name: string) => `轨道名称 ${name}`,
      language: (name: string) => `语言 ${name}`,
      include: (name: string) => `保留 ${name}`,
      default: (name: string) => `默认 ${name}`,
    },
    review: {
      title: '预览',
      description: '确认数据并开始REMUX。',
      source: '片源',
      playlist: '播放列表',
      filename: '文件名',
      outputPath: '输出路径',
      dolbyVisionMergeEnabled: '杜比视界合并已启用',
      yes: '是',
      no: '否',
      finalTrackList: '最终轨道列表与顺序',
      video: '视频',
      audio: '音频',
      subtitle: '字幕',
      defaultFlag: '默认',
      forcedFlag: '强制',
      backButton: '返回',
      startRemuxButton: '开始REMUX',
      startingRemuxButton: '正在REMUX...',
      startNextRemuxButton: '开始新的REMUX',
      currentRemux: '当前任务',
      output: '输出',
      path: '路径',
      progress: '进度',
      commandPreview: '命令预览',
      logOutput: '任务日志',
      waitingForLogOutput: '等待日志输出...',
    },
    app: {
      loginFailed: '登录失败。',
      scanFailed: '片源扫描失败。',
      bdinfoParseFailed: 'BDInfo 解析失败。',
      currentJobRunning: '已有转封装任务在运行，请等待其完成。',
      submitFailed: '启动转封装任务失败。',
    },
  },
  en: {
    layout: {
      appEyebrow: 'Media Mastering Console',
      appTitle: 'MKV Remux Tool',
      appSubtitle: 'A precision workflow for BDInfo parsing, track curation, and remux execution.',
      workflowStepsAria: 'Workflow steps',
      localeToggle: '中文 / EN',
      contextTitle: 'Current Session',
      contextLabels: {
        source: 'Source',
        playlist: 'Playlist',
        output: 'Output',
        task: 'Task',
      },
      readyState: 'Ready',
      lockedState: 'Locked',
      pendingValue: 'Waiting',
      steps: {
        login: 'Login',
        scan: 'Scan',
        bdinfo: 'BDInfo',
        editor: 'Tracks',
        review: 'Review',
      },
    },
    status: {
      running: 'Running',
      succeeded: 'Succeeded',
      failed: 'Failed',
    },
    login: {
      title: 'Login',
      subtitle: 'Single-user local access.',
      passwordLabel: 'Password',
      passwordPlaceholder: 'Enter password',
      continueButton: 'Continue',
      passwordRequired: 'Password is required.',
    },
    scan: {
      title: 'Scan Sources',
      subtitle: 'Only extracted BDMV folders are accepted as workflow input.',
      scanButton: 'Scan Sources (POST /api/sources/scan)',
      scanningButton: 'Scanning...',
      nextButton: 'Continue to BDInfo',
      empty: 'No sources yet. Run scan to discover BDMV directories.',
      columns: {
        select: 'Select',
        name: 'Name',
        type: 'Type',
        path: 'Path',
        size: 'Size',
        modified: 'Modified',
      },
      typeBDMV: 'BDMV Directory',
      selectSource: (name: string) => `Select ${name}`,
    },
    bdinfo: {
      title: 'Required BDInfo',
      selectedSource: 'Selected source',
      description: 'Paste the BDInfo log. This step is required and cannot be skipped.',
      placeholder: 'Paste full BDInfo text here',
      sampleTitle: 'BDInfo Example',
      playlist: 'Playlist',
      audioLabelsFound: 'Audio labels found',
      subtitleLabelsFound: 'Subtitle labels found',
      backButton: 'Back',
      submitButton: 'Parse BDInfo and Continue',
      submittingButton: 'Parsing...',
    },
    editor: {
      title: 'Track Editor',
      titleLabel: 'Title',
      titleHint: 'Recommended: name + year, example: Nightcrawler.2014',
      videoTrackNameLabel: 'Video track name',
      videoSourceAttributes: 'Video source attributes',
      liveFilenamePreview: 'Live filename preview',
      outputFilename: 'Output filename',
      audioHeading: 'Audio',
      subtitlesHeading: 'Subtitles',
      noSubtitles: 'No subtitles found in this draft.',
      backButton: 'Back',
      nextButton: 'Continue to Review',
      columns: {
        drag: 'Move',
        id: 'ID',
        track: 'Track',
        language: 'Language',
        audioFormat: 'Audio Format',
        include: 'Include',
        default: 'Default',
      },
      dragTrack: (name: string) => `Drag ${name}`,
      trackName: (name: string) => `Track name ${name}`,
      language: (name: string) => `Language ${name}`,
      include: (name: string) => `Include ${name}`,
      default: (name: string) => `Default ${name}`,
    },
    review: {
      title: 'Review',
      description: 'Confirm metadata and start the remux.',
      source: 'Source',
      playlist: 'Playlist',
      filename: 'Filename',
      outputPath: 'Output path',
      dolbyVisionMergeEnabled: 'Dolby Vision merge enabled',
      yes: 'Yes',
      no: 'No',
      finalTrackList: 'Final Track List and Order',
      video: 'Video',
      audio: 'Audio',
      subtitle: 'Subtitle',
      defaultFlag: 'default',
      forcedFlag: 'forced',
      backButton: 'Back',
      startRemuxButton: 'Start Remux',
      startingRemuxButton: 'Starting Remux...',
      startNextRemuxButton: 'Start Next Remux',
      currentRemux: 'Current Remux',
      output: 'Output',
      path: 'Path',
      progress: 'Progress',
      commandPreview: 'Command Preview',
      logOutput: 'Task Log',
      waitingForLogOutput: 'Waiting for log output...',
    },
    app: {
      loginFailed: 'Login failed.',
      scanFailed: 'Source scan failed.',
      bdinfoParseFailed: 'BDInfo parse failed.',
      currentJobRunning: 'A remux is already running. Please wait for it to finish.',
      submitFailed: 'Failed to start remux job.',
    },
  },
};

export function getMessages(locale: Locale): MessageSet {
  return messages[locale];
}

export function loadStoredLocale(): Locale {
  if (typeof window === 'undefined') {
    return 'zh';
  }
  const stored = window.localStorage.getItem(localeStorageKey);
  return stored === 'en' ? 'en' : 'zh';
}

export function saveStoredLocale(locale: Locale): void {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.setItem(localeStorageKey, locale);
}

export function loadStoredToken(): string | null {
  if (typeof window === 'undefined') {
    return null;
  }
  return window.localStorage.getItem(tokenStorageKey);
}

export function saveStoredToken(token: string | null): void {
  if (typeof window === 'undefined') {
    return;
  }
  if (token) {
    window.localStorage.setItem(tokenStorageKey, token);
    return;
  }
  window.localStorage.removeItem(tokenStorageKey);
}
