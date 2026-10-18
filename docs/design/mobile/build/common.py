"""Shared CSS, icons and wrappers for the Orion mobile artboards (.dc.html).

Tokens come from docs/design/design-language.md: square corners, flat surfaces,
hairline borders, one green accent, status by shape plus colour.
"""

CSS = """
*{box-sizing:border-box}
body{margin:0}
.m{--gr:#fff;--fd:#f7f7f6;--sf:#fff;--sb:#f7f7f6;--bd:#ececea;--bd2:#d4d4d8;--tx:#18181b;--mu:#52525b;--fa:#71717a;--ac:#1f7a50;--acd:#14573a;--on:#fff;--acs:#e6f4ec;--seg:#f4f4f2;--segon:#fff;
--upb:#ccfbf1;--upt:#115e59;--dnb:#ffe4e6;--dnt:#9f1239;--dgb:#fef3c7;--dgt:#92400e;--mtb:#ede9fe;--mtt:#5b21b6;--unb:#f4f4f2;--unt:#3f3f46;--cvb:#cffafe;--cvt:#155e75;--rsb:#dbeafe;--rst:#1e40af;
--dup:#2a9f69;--ddg:#f59e0b;--dnx:#e5484d;--crit:#f43f5e;--dim:rgba(24,24,27,.4);--csh:0 1px 2px rgba(24,24,27,.05);--bsh:inset 0 1px 0 rgba(255,255,255,.9),0 1px 2px rgba(24,24,27,.08);--psh:inset 0 1px 0 rgba(255,255,255,.22),inset 0 -1px 0 rgba(0,0,0,.14),0 1px 2px rgba(20,87,58,.35);--ish:inset 0 1px 2px rgba(24,24,27,.05);
font-family:"TWK Lausanne",system-ui,-apple-system,"Segoe UI",sans-serif;font-size:15px;line-height:1.4;color:var(--tx);background:var(--gr);width:390px;height:844px;display:flex;flex-direction:column;position:relative;overflow:hidden;font-variant-numeric:tabular-nums;-webkit-font-smoothing:antialiased}
.m.dk{--gr:#111513;--fd:#0b0e0d;--sf:#1a201e;--sb:#161b19;--bd:#252c29;--bd2:#2f3733;--tx:#e9efec;--mu:#b4c0ba;--fa:#8b9892;--ac:#2a9f69;--acd:#14573a;--on:#fff;--acs:#0f2e22;--seg:#0b0e0d;--segon:#222926;
--upb:#0f2e22;--upt:#6ee7b7;--dnb:#3a1720;--dnt:#fda4af;--dgb:#3a2a0a;--dgt:#fcd34d;--mtb:#2a1f4a;--mtt:#c4b5fd;--unb:#161b19;--unt:#b4c0ba;--cvb:#0f2a30;--cvt:#67e8f9;--rsb:#172554;--rst:#93c5fd;
--dup:#2a9f69;--ddg:#f59e0b;--dnx:#e5484d;--crit:#e5484d;--dim:rgba(0,0,0,.6);--csh:inset 0 1px 0 rgba(255,255,255,.03);--bsh:inset 0 1px 0 rgba(255,255,255,.05),0 1px 2px rgba(0,0,0,.5);--psh:inset 0 1px 0 rgba(255,255,255,.18),0 1px 2px rgba(0,0,0,.4);--ish:inset 0 1px 2px rgba(0,0,0,.4)}
.m svg{flex:none}
.hd{display:flex;align-items:center;justify-content:space-between;padding:56px 16px 6px}
.core{display:flex;align-items:center;gap:8px;min-height:44px;padding:0 4px;font-weight:500;color:var(--tx);text-decoration:none}
.core .cv{color:var(--fa)}
.ibtn{width:44px;height:44px;display:flex;align-items:center;justify-content:center;color:var(--tx);text-decoration:none;border:0;background:none;position:relative}
.ttl{font-size:28px;line-height:1.15;font-weight:500;margin:0;padding:4px 16px 12px;letter-spacing:-.01em}
.sub{padding:0 16px 12px;color:var(--fa);font-size:13px;margin-top:-6px}
.sc{flex:1;min-height:0;overflow:hidden;display:flex;flex-direction:column}
.sc>*{flex:none}
.pad{padding:0 16px}
.tabs{height:84px;border-top:1px solid var(--bd);background:var(--gr);display:flex;padding:6px 4px 28px;flex:none}
.tabs a{flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:2px;font-size:11px;color:var(--fa);text-decoration:none;position:relative}
.tabs a.on{color:var(--tx);font-weight:500}
.tabs a.on:after{content:"";position:absolute;top:-7px;left:22%;right:22%;height:2px;background:var(--ac)}
.tabs .bg2{position:absolute;top:0;left:calc(50% + 6px);min-width:18px;height:18px;padding:0 5px;background:var(--crit);color:#fff;font-size:11px;font-weight:500;display:flex;align-items:center;justify-content:center;border-radius:9px}
.card{background:var(--sf);border:1px solid var(--bd);border-radius:10px;box-shadow:var(--csh)}
.cp{padding:14px}
.row{display:flex;align-items:center;gap:10px}
.sp{justify-content:space-between}
.col{display:flex;flex-direction:column}
.g4{gap:4px}.g8{gap:8px}.g12{gap:12px}.g16{gap:16px}
.mu{color:var(--mu)}.fa{color:var(--fa)}.sm{font-size:13px}.xs{font-size:12px}
.mono{font-family:ui-monospace,"SF Mono",Menlo,monospace;font-size:12px;color:var(--fa)}
.bold,.med{font-weight:500}
.b{display:inline-flex;align-items:center;gap:5px;height:20px;padding:0 8px 0 7px;font-size:11.5px;font-weight:500;white-space:nowrap;border-radius:4px}
.b.up{background:var(--upb);color:var(--upt)}.b.dn{background:var(--dnb);color:var(--dnt)}.b.dg{background:var(--dgb);color:var(--dgt)}
.b.mt{background:var(--mtb);color:var(--mtt)}.b.un{background:var(--unb);color:var(--unt)}.b.cv{background:var(--cvb);color:var(--cvt)}
.b.rs{background:var(--rsb);color:var(--rst)}.b.cr{background:var(--crit);color:#fff}
.s{display:inline-block;width:8px;height:8px;flex:none;border-radius:50%}
.s.up{background:var(--dup)}.s.dn{background:var(--dnx)}.s.un{background:#a1a1aa}.s.dg{background:var(--ddg)}.s.mt{background:#8b5cf6}
.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;min-height:44px;padding:0 16px;border:1px solid var(--bd2);border-radius:6px;background:var(--sf);color:var(--tx);font:inherit;font-weight:500;text-decoration:none;white-space:nowrap;box-shadow:var(--bsh)}
.btn.p{background:var(--ac);border-color:var(--acd);color:var(--on);box-shadow:var(--psh)}
.btn.d{background:var(--dnx);border-color:#b42318;color:#fff}
.btn.g{border-color:transparent;background:none;box-shadow:none}
.btn.sm{min-height:36px;padding:0 12px;font-size:13px}
.btn.f{flex:1}
.chips{display:flex;gap:8px;padding:0 16px 12px;overflow:hidden}
.chip{display:inline-flex;align-items:center;gap:6px;height:32px;padding:0 12px;border-radius:6px;background:var(--sf);font-size:13px;font-weight:500;white-space:nowrap;color:var(--tx);box-shadow:var(--bsh);border:1px solid var(--bd)}
.chip.on{border-color:var(--ac);background:var(--acs);color:var(--ac)}
.seg{display:flex;margin:0 16px 12px;padding:3px;border-radius:8px;background:var(--seg);box-shadow:inset 0 1px 2px rgba(24,24,27,.08)}
.seg a{flex:1;min-height:36px;display:flex;align-items:center;justify-content:center;gap:6px;font-size:13px;color:var(--mu);text-decoration:none;border-radius:6px}
.seg a.on{background:var(--segon);color:var(--tx);font-weight:500;box-shadow:var(--bsh)}
.ut{display:flex;gap:0;margin:0 16px 12px;border-bottom:1px solid var(--bd)}
.ut a{flex:1;min-height:44px;display:flex;align-items:center;justify-content:center;font-size:14px;color:var(--mu);text-decoration:none;position:relative}
.ut a.on{color:var(--tx);font-weight:500}
.ut a.on:after{content:"";position:absolute;left:0;right:0;bottom:-1px;height:2px;background:var(--ac)}
.fld{display:flex;align-items:center;gap:8px;min-height:48px;padding:0 12px;border:1px solid var(--bd2);border-radius:6px;background:var(--sf);color:var(--tx);box-shadow:var(--ish)}
.fld.f{border-color:var(--ac);box-shadow:0 0 0 3px var(--acs)}
.fld input,.fld span.v{flex:1;border:0;background:none;color:var(--tx);font:inherit;min-width:0;outline:0}
.lbl{font-size:13px;font-weight:500;margin-bottom:6px;display:block}
.hint{font-size:12px;color:var(--fa);margin-top:6px}
.list{background:var(--sf);border:1px solid var(--bd);border-radius:10px;box-shadow:var(--csh);margin:0 16px;overflow:hidden}
.li{display:flex;align-items:center;gap:12px;min-height:56px;padding:10px 14px;border-top:1px solid var(--bd);color:var(--tx);text-decoration:none}
.li:first-child{border-top:0}
.li .t{flex:1;min-width:0}
.li .t b{font-weight:500;display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.li .t small{display:block;color:var(--fa);font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.sec{padding:16px 16px 8px;font-size:12px;font-weight:500;color:var(--fa)}
.unr{width:8px;height:8px;background:var(--ac);border-radius:50%;flex:none}
.bar{display:flex;gap:8px;padding:12px 16px 30px;background:var(--gr);border-top:1px solid var(--bd);flex:none}
.scrim{position:absolute;inset:0;background:var(--dim)}
.sheet{position:absolute;left:0;right:0;bottom:0;background:var(--gr);border-top:1px solid var(--bd);border-radius:14px 14px 0 0;padding:8px 16px 34px;display:flex;flex-direction:column;gap:14px;box-shadow:0 -8px 24px rgba(0,0,0,.12)}
.grab{width:36px;height:4px;background:var(--bd2);border-radius:2px;align-self:center;margin-bottom:4px}
.sh1{font-size:18px;font-weight:500}
.ban{display:flex;align-items:center;gap:10px;padding:10px 16px;font-size:13px}
.ban.dg{background:var(--dgb);color:var(--dgt)}.ban.dn{background:var(--dnb);color:var(--dnt)}.ban.un{background:var(--fd);color:var(--mu);border-bottom:1px solid var(--bd)}
.call{display:flex;gap:10px;padding:12px;font-size:13px;border:1px solid var(--bd);border-radius:8px;background:var(--fd)}
.call.dg{background:var(--dgb);color:var(--dgt);border-color:transparent}
.call.up{background:var(--upb);color:var(--upt);border-color:transparent}
.st4{display:grid;grid-template-columns:repeat(3,1fr);border:1px solid var(--bd);border-radius:10px;background:var(--sf);box-shadow:var(--csh);overflow:hidden}
.st4>div{padding:10px 12px;border-left:1px solid var(--bd)}.st4>div:first-child{border-left:0}
.st4 span{display:block;font-size:12px;color:var(--fa)}.st4 b{font-size:20px;font-weight:500}.st4 small{font-size:12px;color:var(--fa);font-weight:400}
.tl{display:flex;flex-direction:column}
.tl>div{display:flex;gap:12px;padding-bottom:16px;position:relative}
.tl>div:before{content:"";position:absolute;left:3px;top:14px;bottom:0;width:1px;background:var(--bd)}
.tl>div:last-child:before{display:none}
.tl i{margin-top:6px}
.uptime{display:flex;gap:2px;height:28px;align-items:stretch}
.uptime i{flex:1;background:var(--dup);border-radius:1px}
.uptime i.d{background:var(--dnx)}.uptime i.w{background:var(--ddg)}
.log{display:flex;gap:10px;padding:10px 16px;border-top:1px solid var(--bd);font-size:13px}
.log time{font-family:ui-monospace,Menlo,monospace;font-size:11px;color:var(--fa);flex:none;width:58px;padding-top:2px}
.log p{margin:0;flex:1;min-width:0;overflow:hidden;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;word-break:break-word}
.lv{font-size:11px;font-weight:500;padding:1px 6px;flex:none;height:18px;margin-top:1px;border-radius:4px}
.lv.e{background:var(--dnb);color:var(--dnt)}.lv.w{background:var(--dgb);color:var(--dgt)}.lv.i{background:var(--unb);color:var(--unt)}
.tg{width:44px;height:26px;background:var(--bd2);position:relative;flex:none;border-radius:13px}
.tg:after{content:"";position:absolute;top:3px;left:3px;width:20px;height:20px;background:#fff;border-radius:50%;box-shadow:0 1px 2px rgba(0,0,0,.25)}
.tg.on{background:var(--ac)}.tg.on:after{left:21px}
.cam{background:#0c0a09;color:#fafaf9}
.qr{width:240px;height:240px;position:relative}
.qr i{position:absolute;width:36px;height:36px;border:3px solid #fafaf9;border-radius:4px}
.ctr{display:flex;flex-direction:column;align-items:center;text-align:center}
.hero{font-family:"Instrument Serif",Georgia,serif;font-size:40px;line-height:1.05;font-weight:400;letter-spacing:-.01em}
.logo{width:56px;height:56px;background:var(--ac);color:#fff;display:flex;align-items:center;justify-content:center;border-radius:14px;box-shadow:inset 0 1px 0 rgba(255,255,255,.25),0 1px 2px rgba(0,0,0,.2)}
.lock{background:linear-gradient(#1f2a26,#0c0a09);color:#fafaf9}
.ntf{background:rgba(250,250,249,.14);padding:12px;display:flex;flex-direction:column;gap:2px;border:1px solid rgba(250,250,249,.12);border-radius:14px}

.mk{display:inline-flex;align-items:center;justify-content:center;background:var(--ac);color:#fff;border-radius:8px;box-shadow:inset 0 1px 0 rgba(255,255,255,.25),0 1px 2px rgba(0,0,0,.2)}
.btn.p{background:linear-gradient(#27905f,#1f7a50);}
.m.dk .btn.p{background:linear-gradient(#2a9f69,#1f7a50)}
.b:before{content:"";width:6px;height:6px;background:currentColor;border-radius:50%;flex:none}
.b.dn:before{border-radius:0}
.hp{display:inline-flex;align-items:center;gap:7px;height:34px;padding:0 10px;border:1px solid var(--bd);border-radius:8px;background:var(--sf);box-shadow:var(--csh);font-size:12.5px;font-weight:500}
.ibtn.sq{width:36px;height:34px;margin-right:8px;border:1px solid var(--bd);border-radius:8px;background:var(--sf);box-shadow:var(--csh)}
.nav{padding:8px 12px 26px;background:var(--gr);flex:none;border-top:1px solid var(--bd)}
.tray{display:flex;gap:2px;padding:4px;border-radius:14px;background:var(--seg);box-shadow:inset 0 1px 2px rgba(24,24,27,.08)}
.tray a{flex:1;height:50px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:2px;font-size:10.5px;font-weight:500;color:var(--fa);text-decoration:none;border-radius:10px;position:relative}
.tray a.on{background:var(--segon);color:var(--tx);box-shadow:var(--bsh)}
.tray .bg2{position:absolute;top:3px;left:calc(50% + 4px);min-width:17px;height:17px;padding:0 4px;background:var(--crit);color:#fff;font-size:10.5px;font-weight:500;display:flex;align-items:center;justify-content:center;border-radius:9px}
.fieldbox{background:var(--fd);border:1px solid var(--bd);border-radius:14px;padding:10px;margin:0 16px}
.fh{display:flex;justify-content:space-between;align-items:center;padding:2px 6px 10px;font-size:13.5px}
.fh b{font-weight:500}.fh span{color:var(--fa);font-size:12.5px}
.trk{position:relative;display:grid;grid-template-columns:repeat(4,1fr);text-align:center;padding-top:2px}
.trk .ln,.trk .fl{position:absolute;top:13px;left:12.5%;height:2px;background:var(--bd)}.trk .ln{width:75%}.trk .fl{background:var(--dup);z-index:1}
.trk>div{position:relative;z-index:2;display:flex;flex-direction:column;align-items:center;gap:1px}
.trk i{width:22px;height:22px;border-radius:50%;background:var(--sf);border:2px solid var(--bd2);display:flex;align-items:center;justify-content:center;color:#fff;font-style:normal}
.trk .done i{background:var(--dup);border-color:var(--dup)}
.trk .now i{border-color:var(--dnx);box-shadow:0 0 0 4px rgba(229,72,77,.16)}
.trk .nowa i{border-color:var(--ddg);box-shadow:0 0 0 4px rgba(245,158,11,.2)}
.trk b{font-size:12.5px;font-weight:500;margin-top:6px}.trk small{font-size:11px;color:var(--fa)}
.ea{display:grid;grid-template-columns:1fr 1fr;gap:8px;margin:0 16px}
.ea>div{border-radius:12px;padding:14px;display:flex;flex-direction:column;gap:3px}
.ea .x{background:#142a23;color:#fff}.m.dk .ea .x{background:#0a1f18;border:1px solid #14573a}
.ea .x span{color:#80ffaa}
.ea span{font-size:10.5px;letter-spacing:.08em;text-transform:uppercase;font-weight:500}
.ea .y{background:var(--sf);border:1px solid var(--bd);box-shadow:var(--csh)}.ea .y span{color:var(--fa)}
.ea b{font-size:22px;font-weight:400;letter-spacing:-.01em;line-height:1.15}.ea small{font-size:12px;opacity:.72}
.box{margin:0 16px;background:var(--sf);border:1px solid var(--bd);border-radius:10px;box-shadow:var(--csh);overflow:hidden}
.bh{display:flex;justify-content:space-between;align-items:center;padding:11px 14px;background:var(--fd);border-bottom:1px solid var(--bd);font-size:13.5px}
.bh b{font-weight:500}.bh span{color:var(--fa);font-size:12.5px}
.br{display:flex;align-items:center;gap:10px;padding:12px 14px;border-top:1px solid var(--bd);color:var(--tx);text-decoration:none;min-height:52px}
.bh+.br{border-top:0}
.br .t{flex:1;min-width:0}.br .t b{font-weight:500;display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.br .t small{display:block;font-size:12px;color:var(--fa)}
.br .pc{font-size:12.5px;color:var(--mu);width:46px;text-align:right}
.ub{display:flex;gap:1px;height:18px;align-items:stretch}.ub i{width:3px;background:var(--dup);border-radius:1px}.ub i.d{background:var(--dnx)}.ub i.w{background:var(--ddg)}
.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:8px;margin:0 16px}
.sc3{padding:10px 12px}.sc3 span{font-size:12px;color:var(--fa);display:block}.sc3 b{font-size:22px;font-weight:500;letter-spacing:-.01em}.sc3 small{font-size:12px;color:var(--fa);font-weight:400}
.tl2>div{display:flex;gap:12px;align-items:flex-start;padding:9px 0}
.tl2 p{margin:1px 0 0;flex:1;font-size:14px;line-height:1.4}.tl2 time{font-size:12px;color:var(--fa);padding-top:3px}
.td{width:22px;height:22px;border-radius:50%;display:flex;align-items:center;justify-content:center;flex:none;background:var(--unb)}
.td i{width:8px;height:8px;border-radius:50%;background:#a1a1aa}
.td.dn{background:rgba(229,72,77,.14)}.td.dn i{background:var(--dnx)}.td.dg{background:rgba(245,158,11,.18)}.td.dg i{background:var(--ddg)}.td.up{background:rgba(42,159,105,.16)}.td.up i{background:var(--dup)}
.avt{width:26px;height:26px;border-radius:50%;background:conic-gradient(#f59e0b,#e5484d,#8b5cf6,#3b82f6,#2a9f69,#f59e0b);flex:none}
.crumb{display:flex;align-items:center;gap:6px;color:var(--fa);font-size:13px}
.h2t{font-size:26px;line-height:1.15;font-weight:500;margin:0;letter-spacing:-.015em}
.prose{font-size:14.5px;line-height:1.5;margin:0;color:var(--tx)}.prose b{font-weight:500}
.deep{background:#142a23;color:#fff}
"""

ICONS = {
 "bell": '<path d="M18 8a6 6 0 1 0-12 0c0 7-3 8-3 8h18s-3-1-3-8"/><path d="M10.3 21a2 2 0 0 0 3.4 0"/>',
 "alert": '<path d="M12 3l10 18H2z"/><path d="M12 10v5M12 18h.01"/>',
 "server": '<rect x="3" y="4" width="18" height="7"/><rect x="3" y="13" width="18" height="7"/><path d="M7 7.5h.01M7 16.5h.01"/>',
 "pulse": '<path d="M3 12h4l3-8 4 16 3-8h4"/>',
 "more": '<circle cx="5" cy="12" r="1.2"/><circle cx="12" cy="12" r="1.2"/><circle cx="19" cy="12" r="1.2"/>',
 "search": '<circle cx="11" cy="11" r="7"/><path d="M20 20l-4-4"/>',
 "chev": '<path d="M9 6l6 6-6 6"/>',
 "down": '<path d="M6 9l6 6 6-6"/>',
 "back": '<path d="M15 6l-6 6 6 6"/>',
 "check": '<path d="M5 12l5 5 9-10"/>',
 "x": '<path d="M6 6l12 12M18 6L6 18"/>',
 "filter": '<path d="M3 5h18l-7 8v6l-4-2v-4z"/>',
 "pause": '<path d="M8 5v14M16 5v14"/>',
 "play": '<path d="M7 4l13 8-13 8z"/>',
 "refresh": '<path d="M20 11a8 8 0 1 0-2.3 5.7M20 4v7h-7"/>',
 "shield": '<path d="M12 3l8 3v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z"/>',
 "moon": '<path d="M20 14.5A8 8 0 0 1 9.5 4 8 8 0 1 0 20 14.5z"/>',
 "face": '<path d="M4 8V6a2 2 0 0 1 2-2h2M16 4h2a2 2 0 0 1 2 2v2M20 16v2a2 2 0 0 1-2 2h-2M8 20H6a2 2 0 0 1-2-2v-2M9 10v1M15 10v1M9 15c1.5 1.5 4.5 1.5 6 0"/>',
 "qr": '<rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><path d="M14 14h3v3h-3zM20 14v.01M14 20h3M20 17v4"/>',
 "link": '<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3A4 4 0 0 0 11 18.7l1-1"/>',
 "user": '<circle cx="12" cy="8" r="4"/><path d="M4 21c0-4 4-6 8-6s8 2 8 6"/>',
 "log": '<path d="M5 3h10l4 4v14H5z"/><path d="M9 11h6M9 15h6"/>',
 "send": '<path d="M4 12l16-8-6 16-3-7z"/>',
 "clock": '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
 "off": '<path d="M2 2l20 20M8.5 8.5A8 8 0 0 0 5 11M2 8a14 14 0 0 1 5-3M12 20h.01M10 16a4 4 0 0 1 4 0M19 11a8 8 0 0 0-3-2"/>',
 "mail": '<rect x="3" y="5" width="18" height="14"/><path d="M3 7l9 6 9-6"/>',
 "hook": '<path d="M10 13a3 3 0 1 1 4-4M8 14a4 4 0 1 0 3 6.5M16 18h4M12 8l3 5 4-1"/>',
 "phone": '<rect x="7" y="2" width="10" height="20"/><path d="M11 18h2"/>',
 "inbox": '<path d="M3 13l3-8h12l3 8v6H3z"/><path d="M3 13h5l1 3h6l1-3h5"/>',
 "gear": '<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M19 5l-2 2M7 17l-2 2"/>',
 "flash": '<path d="M13 2L4 14h7l-1 8 9-12h-7z"/>',
 "plus": '<path d="M12 5v14M5 12h14"/>',
 "note": '<path d="M4 4h16v12l-5 5H4z"/><path d="M15 21v-5h5M8 9h8M8 13h4"/>',
 "globe": '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18"/>',
 "copy": '<rect x="8" y="8" width="12" height="12"/><path d="M16 8V4H4v12h4"/>',
 "trash": '<path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13"/>',
}


def ic(name, size=20, sw=1.7, extra=""):
    return ('<svg width="%d" height="%d" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="%s" '
            'stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" %s>%s</svg>' % (size, size, sw, extra, ICONS[name]))


def page(title, body, dark, w=390, h=844, extra_css="", root_class="m"):
    cls = root_class + (" dk" if dark else "")
    return ('<!doctype html>\n<html lang="en"><head><meta charset="utf-8"><title>%s</title>'
            '<script src="./support.js"></script></head>\n<body><x-dc><helmet>'
            '<link href="https://fonts.googleapis.com/css2?family=Instrument+Serif&amp;display=swap" rel="stylesheet">'
            '<style>%s%s</style></helmet>\n<div class="%s" style="width:%dpx;height:%dpx">%s</div></x-dc>\n'
            '<script type="text/x-dc" data-dc-script data-props=\'{"$preview":{"width":%d,"height":%d}}\'>'
            'class Component extends DCLogic{renderVals(){return{}}}</script></body></html>\n'
            % (title, CSS, extra_css, cls, w, h, body, w, h))
