import { z } from 'zod';

export const NodeRecordSchema = z.object({
  id: z.number(),
  name: z.string().optional(),
  remark: z.string().optional(),
  scheme: z.string().optional(),
  address: z.string().optional(),
  port: z.number().optional(),
  basePath: z.string().optional(),
  apiToken: z.string().optional(),
  enable: z.boolean().optional(),
  status: z.string().optional(),
  latencyMs: z.number().optional(),
  cpuPct: z.number().optional(),
  memPct: z.number().optional(),
  xrayVersion: z.string().optional(),
  panelVersion: z.string().optional(),
  uptimeSecs: z.number().optional(),
  inboundCount: z.number().optional(),
  clientCount: z.number().optional(),
  onlineCount: z.number().optional(),
  depletedCount: z.number().optional(),
  lastHeartbeat: z.number().optional(),
  lastError: z.string().optional(),
  allowPrivateAddress: z.boolean().optional(),
  // 这台节点机自己的域名和证书。空 = 该节点没配证书,需要证书的一键模板
  // 就不能部署到它上面(以前会静默套用面板自己的证书,那台机上根本没有那个文件)
  domain: z.string().optional(),
  certFile: z.string().optional(),
  keyFile: z.string().optional(),
}).loose();

export const NodeListSchema = z.array(NodeRecordSchema);

export const ProbeResultSchema = z.object({
  status: z.string(),
  latencyMs: z.number().optional(),
  xrayVersion: z.string().optional(),
  error: z.string().optional(),
}).loose();

export const NodeFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  remark: z.string().optional(),
  scheme: z.enum(['http', 'https']),
  address: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  port: z.number().int().min(1).max(65535),
  basePath: z.string(),
  apiToken: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  enable: z.boolean(),
  allowPrivateAddress: z.boolean(),
  // 选填:留空表示这台节点上没有证书。需要证书的模板会据此拒绝部署并说明原因,
  // 而不是拿面板自己的证书顶上去。
  domain: z.string().optional(),
  certFile: z.string().optional(),
  keyFile: z.string().optional(),
});

export type NodeRecord = z.infer<typeof NodeRecordSchema>;
export type ProbeResult = z.infer<typeof ProbeResultSchema>;
export type NodeFormValues = z.infer<typeof NodeFormSchema>;
