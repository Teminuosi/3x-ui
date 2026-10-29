/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import {
  RELAY_ENTRY_PRESETS,
  RELAY_ENTRY_SNIFFING,
  applyRelayToTemplate,
  buildRelayInboundPayload,
  buildRelayRule,
  isFragileRelayLanding,
  isRelayEntryTag,
  landingOutboundFromLink,
  landingOutboundFromManual,
  parseLandingEndpoint,
  uniqueOutboundTag,
  type LandingManualInput,
} from '@/lib/xray/relay';
import { formValuesToWirePayload, rawInboundToFormValues } from '@/lib/xray/inbound-form-adapter';
import { INBOUND_PRESETS } from '@/lib/xray/inbound-presets';
import { HttpOutboundSettingsSchema } from '@/schemas/protocols/outbound/http';
import { ShadowsocksOutboundSettingsSchema } from '@/schemas/protocols/outbound/shadowsocks';
import { SocksOutboundSettingsSchema } from '@/schemas/protocols/outbound/socks';
import { TrojanOutboundSettingsSchema } from '@/schemas/protocols/outbound/trojan';
import { VlessOutboundSettingsSchema } from '@/schemas/protocols/outbound/vless';
import { VmessOutboundSettingsSchema } from '@/schemas/protocols/outbound/vmess';
import { RuleObjectSchema } from '@/schemas/routing';
import { XraySettingsValueSchema } from '@/schemas/xray';

describe('relay landing outbound (manual)', () => {
  const base = { address: '203.0.113.9', port: 1080 };

  const cases: { input: LandingManualInput; schema: { safeParse: (v: unknown) => { success: boolean } } }[] = [
    { input: { ...base, protocol: 'socks', port: 1080, user: 'u', pass: 'p' }, schema: SocksOutboundSettingsSchema },
    { input: { ...base, protocol: 'http', port: 8080, user: 'u', pass: 'p' }, schema: HttpOutboundSettingsSchema },
    { input: { ...base, protocol: 'vless', port: 443, id: '8c14d6f7-2e3b-4a91-9d24-3f7a6b8c1e02' }, schema: VlessOutboundSettingsSchema },
    { input: { ...base, protocol: 'vmess', port: 443, id: '8c14d6f7-2e3b-4a91-9d24-3f7a6b8c1e02' }, schema: VmessOutboundSettingsSchema },
    { input: { ...base, protocol: 'trojan', port: 443, password: 'secret' }, schema: TrojanOutboundSettingsSchema },
    { input: { ...base, protocol: 'shadowsocks', port: 443, password: 'secret', method: '2022-blake3-aes-256-gcm' }, schema: ShadowsocksOutboundSettingsSchema },
  ];

  for (const { input, schema } of cases) {
    it(`${input.protocol} builds a schema-valid outbound`, () => {
      const ob = landingOutboundFromManual(input, `relay-${input.protocol}`);
      expect(ob.tag).toBe(`relay-${input.protocol}`);
      expect(ob.protocol).toBe(input.protocol);
      const parsed = schema.safeParse(ob.settings);
      if (!parsed.success) throw new Error(`${input.protocol}: ${JSON.stringify(ob.settings)}`);
      expect(parsed.success).toBe(true);
    });
  }

  it('socks without credentials yields an empty users array', () => {
    const ob = landingOutboundFromManual({ ...base, protocol: 'socks' }, 'relay-socks');
    const settings = ob.settings as { servers: { users: unknown[] }[] };
    expect(settings.servers[0].users).toEqual([]);
  });
});

describe('relay landing outbound (link)', () => {
  it('parses a vless share link into an outbound, overriding the tag', () => {
    const link =
      'vless://8c14d6f7-2e3b-4a91-9d24-3f7a6b8c1e02@198.51.100.7:443?encryption=none&security=reality&type=tcp#landing-node';
    const ob = landingOutboundFromLink(link, 'relay-landing');
    expect(ob).not.toBeNull();
    expect(ob!.protocol).toBe('vless');
    expect(ob!.tag).toBe('relay-landing'); // remark from link is discarded
  });

  it('returns null for an unparseable link', () => {
    expect(landingOutboundFromLink('not-a-link', 'relay-x')).toBeNull();
  });
});

describe('relay entry inbound payload', () => {
  it('entry presets are all cert-free (no domain needed)', () => {
    expect(RELAY_ENTRY_PRESETS.length).toBeGreaterThan(0);
    expect(RELAY_ENTRY_PRESETS.every((p) => !p.needsDomain)).toBe(true);
  });

  it('applies remark/port and injects the reality key pair', () => {
    const preset = RELAY_ENTRY_PRESETS[0];
    const payload = buildRelayInboundPayload(preset, {
      remark: '中转-东京',
      port: 23456,
      realityPrivateKey: 'PRIV_KEY',
      realityPublicKey: 'PUB_KEY',
    });
    expect(payload.remark).toBe('中转-东京');
    expect(payload.port).toBe(23456);
    const stream = JSON.parse(payload.streamSettings) as {
      realitySettings: { privateKey: string; settings: { publicKey: string } };
    };
    expect(stream.realitySettings.privateKey).toBe('PRIV_KEY');
    expect(stream.realitySettings.settings.publicKey).toBe('PUB_KEY');
  });

  it('carries exactly one client in the entry inbound', () => {
    const payload = buildRelayInboundPayload(RELAY_ENTRY_PRESETS[0], { port: 20000 });
    const settings = JSON.parse(payload.settings) as { clients: unknown[] };
    expect(settings.clients).toHaveLength(1);
  });

  it('tags the entry inbound with the relay prefix so lists can badge it', () => {
    const payload = buildRelayInboundPayload(RELAY_ENTRY_PRESETS[0], { port: 20000 });
    expect(payload.tag).toBe('relay-in-20000');
    expect(isRelayEntryTag(payload.tag)).toBe(true);
  });

  it('isRelayEntryTag distinguishes relay entries from normal inbounds', () => {
    expect(isRelayEntryTag('relay-in-12345')).toBe(true);
    expect(isRelayEntryTag('inbound-443')).toBe(false);
    expect(isRelayEntryTag('')).toBe(false);
    expect(isRelayEntryTag(undefined)).toBe(false);
  });
});

describe('parseLandingEndpoint', () => {
  it('parses host:port', () => {
    expect(parseLandingEndpoint('1.2.3.4:1080')).toEqual({ address: '1.2.3.4', port: 1080 });
  });
  it('parses host:port:user:pass (residential 4-tuple)', () => {
    expect(parseLandingEndpoint('178.210.253.190:12324:14a6a331454a2:8d9f8f7cc3')).toEqual({
      address: '178.210.253.190', port: 12324, user: '14a6a331454a2', pass: '8d9f8f7cc3',
    });
  });
  it('parses user:pass@host:port', () => {
    expect(parseLandingEndpoint('u:p@1.2.3.4:1080')).toEqual({
      address: '1.2.3.4', port: 1080, user: 'u', pass: 'p',
    });
  });
  it('parses bracketed IPv6 host:port', () => {
    expect(parseLandingEndpoint('[2001:db8::1]:1080')).toEqual({ address: '2001:db8::1', port: 1080 });
  });
  it('returns null for a bare hostname (no port)', () => {
    expect(parseLandingEndpoint('example.com')).toBeNull();
  });
  it('returns null for a share link (handled elsewhere)', () => {
    expect(parseLandingEndpoint('vless://uuid@host:443')).toBeNull();
  });
  it('rejects an invalid port', () => {
    expect(parseLandingEndpoint('1.2.3.4:99999')).toBeNull();
  });
});

describe('relay rule + template splice', () => {
  it('builds a routing rule that passes RuleObjectSchema', () => {
    const rule = buildRelayRule('inbound-tcp-12345', 'relay-landing');
    const parsed = RuleObjectSchema.safeParse(rule);
    expect(parsed.success).toBe(true);
    expect(rule.inboundTag).toEqual(['inbound-tcp-12345']);
    expect(rule.outboundTag).toBe('relay-landing');
  });

  it('appends outbound + prepends rule without mutating the input template', () => {
    const template = {
      outbounds: [{ tag: 'direct', protocol: 'freedom', settings: {} }],
      routing: { rules: [{ type: 'field' as const, outboundTag: 'direct' }] },
    };
    const ob = landingOutboundFromManual({ protocol: 'socks', address: '203.0.113.9', port: 1080 }, 'relay-socks');
    const rule = buildRelayRule('inbound-1', 'relay-socks');
    const next = applyRelayToTemplate(template, ob, rule);

    // Immutable: original untouched.
    expect(template.outbounds).toHaveLength(1);
    expect(template.routing.rules).toHaveLength(1);
    // Next has both, rule prepended (first-match-wins).
    expect(next.outbounds).toHaveLength(2);
    expect(next.routing!.rules).toHaveLength(2);
    expect(next.routing!.rules![0]).toEqual(rule);
    // Whole template still validates.
    expect(XraySettingsValueSchema.safeParse(next).success).toBe(true);
  });

  it('uniqueOutboundTag dedupes against existing tags', () => {
    expect(uniqueOutboundTag(['direct', 'blocked'], 'relay-socks')).toBe('relay-socks');
    expect(uniqueOutboundTag(['relay-socks'], 'relay-socks')).toBe('relay-socks-2');
    expect(uniqueOutboundTag(['relay-socks', 'relay-socks-2'], 'relay-socks')).toBe('relay-socks-3');
  });
});

describe('relay entry sniffing (zero-length AEAD chunk workaround)', () => {
  for (const preset of RELAY_ENTRY_PRESETS) {
    it(`${preset.id}: relay entry ships sniffing on, routeOnly, http/tls/quic`, () => {
      const payload = buildRelayInboundPayload(preset, { port: 20000 });
      expect(JSON.parse(payload.sniffing)).toEqual({
        enabled: true,
        destOverride: ['http', 'tls', 'quic'],
        routeOnly: true,
      });
    });
  }

  it('never rewrites the destination (routeOnly, no fakedns)', () => {
    expect(RELAY_ENTRY_SNIFFING.routeOnly).toBe(true);
    expect(RELAY_ENTRY_SNIFFING.destOverride).not.toContain('fakedns');
  });

  it('plain one-click presets keep sniffing off (only relay entries change)', () => {
    for (const preset of INBOUND_PRESETS) {
      const payload = formValuesToWirePayload(rawInboundToFormValues(preset.build('example.com')) as never);
      expect(JSON.parse(payload.sniffing)).toEqual({ enabled: false });
    }
  });

  it('does not mutate the shared sniffing constant', () => {
    buildRelayInboundPayload(RELAY_ENTRY_PRESETS[0], { port: 20001 });
    expect(RELAY_ENTRY_SNIFFING).toEqual({ enabled: true, destOverride: ['http', 'tls', 'quic'], routeOnly: true });
  });
});

describe('isFragileRelayLanding', () => {
  const ss = (method: string) =>
    landingOutboundFromManual({ protocol: 'shadowsocks', address: '203.0.113.9', port: 8388, password: 'x', method }, 'r');

  it('flags VMess and legacy (non-2022) Shadowsocks', () => {
    expect(isFragileRelayLanding(landingOutboundFromManual({ protocol: 'vmess', address: '203.0.113.9', port: 443, id: 'u' }, 'r'))).toBe(true);
    expect(isFragileRelayLanding(ss('aes-256-gcm'))).toBe(true);
    expect(isFragileRelayLanding(ss('aes-128-gcm'))).toBe(true);
    expect(isFragileRelayLanding(ss('chacha20-ietf-poly1305'))).toBe(true);
  });

  it('does not flag SS-2022 / VLESS / Trojan / SOCKS / HTTP / Hysteria2', () => {
    expect(isFragileRelayLanding(ss('2022-blake3-aes-256-gcm'))).toBe(false);
    expect(isFragileRelayLanding(ss('2022-blake3-aes-128-gcm'))).toBe(false);
    for (const protocol of ['vless', 'trojan', 'socks', 'http'] as const) {
      expect(isFragileRelayLanding(landingOutboundFromManual({ protocol, address: '203.0.113.9', port: 443 }, 'r'))).toBe(false);
    }
    expect(isFragileRelayLanding({ protocol: 'hysteria', settings: {} })).toBe(false);
    expect(isFragileRelayLanding(null)).toBe(false);
  });

  it('reads the cipher from a pasted ss:// link', () => {
    const legacy = 'ss://' + btoa('aes-256-gcm:secret') + '@198.51.100.7:8388#land';
    const modern = 'ss://' + btoa('2022-blake3-aes-256-gcm:AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=') + '@198.51.100.7:8388#land';
    expect(isFragileRelayLanding(landingOutboundFromLink(legacy, 'r'))).toBe(true);
    expect(isFragileRelayLanding(landingOutboundFromLink(modern, 'r'))).toBe(false);
  });
});
