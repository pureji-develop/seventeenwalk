import * as Phaser from 'phaser';
import { FONT_DISPLAY, FONT_UI, PHASER_COLOR, UI_COLOR } from '../constants/theme';

export class CreditScene extends Phaser.Scene {
  constructor() {
    super('CreditScene');
  }

  create() {
    const { width, height } = this.scale;
    const landscapeCompact = width / height >= 1.25 || height <= 520;
    const marginX = landscapeCompact ? 36 : 52;
    const panelPadY = landscapeCompact ? 14 : 20;
    const wrapWidth = landscapeCompact
      ? Math.floor((width - marginX * 2 - 28) / 2) - 12
      : width - marginX * 2;

    const panel = this.add.graphics();
    panel.fillStyle(PHASER_COLOR.panel, 0.82);
    panel.fillRoundedRect(28, panelPadY, width - 56, height - panelPadY * 2, 18);
    panel.lineStyle(1, 0xe8eef4, 0.1);
    panel.strokeRoundedRect(28, panelPadY, width - 56, height - panelPadY * 2, 18);

    let y = panelPadY + 18;
    const titleSize = landscapeCompact ? '22px' : '28px';
    const headSize = landscapeCompact ? '13px' : '15px';
    const bodySize = landscapeCompact ? '12px' : '13px';
    const lineSp = landscapeCompact ? 4 : 6;

    const title = this.add.text(width / 2, y, 'Credits & Disclaimer', {
      fontFamily: FONT_DISPLAY,
      fontSize: titleSize,
      color: UI_COLOR.ivory,
      align: 'center',
    });
    title.setOrigin(0.5, 0);
    y += landscapeCompact ? 34 : 46;

    const disclaimer =
      '本作は二人麻雀「十七歩（じゅうななほ）」を題材とした非営利のファンメイド対戦デモです。\n' +
      'ゲームロジックとリアルタイム通信の学習・検証を目的としており、商業利用を行う予定はありません。\n' +
      '麻雀のルール・用語は伝統的な慣例および本プロジェクトの設計書に基づきます。';

    const tech =
      '• Backend: Go, WebSocket（インメモリ・2人対戦）\n' +
      '• Frontend: Phaser 3, TypeScript, Vite\n' +
      '• Infra: Docker\n' +
      '• Development Assistant: AI（Cursor）— 人間との共同開発';

    const thanks =
      '• 賭博堕天録カイジ — 原作者\n' +
      '• 麻雀王国 — 牌画像の出典元';

    if (landscapeCompact) {
      const colGap = 28;
      const leftX = marginX;
      const rightX = marginX + wrapWidth + colGap;
      let leftY = y;
      let rightY = y;

      this.add.text(leftX, leftY, '免責事項 / Disclaimer', {
        fontFamily: FONT_UI,
        fontSize: headSize,
        fontStyle: 'bold',
        color: UI_COLOR.accentSoft,
      });
      leftY += 22;
      const dText = this.add.text(leftX, leftY, disclaimer, {
        fontFamily: FONT_UI,
        fontSize: bodySize,
        color: UI_COLOR.muted,
        lineSpacing: lineSp,
        wordWrap: { width: wrapWidth, useAdvancedWrap: true },
      });
      leftY += dText.height + 14;

      this.add.text(rightX, rightY, '技術スタック / Technology Stack', {
        fontFamily: FONT_UI,
        fontSize: headSize,
        fontStyle: 'bold',
        color: UI_COLOR.accentSoft,
      });
      rightY += 22;
      const tText = this.add.text(rightX, rightY, tech, {
        fontFamily: FONT_UI,
        fontSize: bodySize,
        color: UI_COLOR.muted,
        lineSpacing: lineSp + 2,
        wordWrap: { width: wrapWidth, useAdvancedWrap: true },
      });
      rightY += tText.height + 14;

      this.add.text(rightX, rightY, 'Special Thanks', {
        fontFamily: FONT_UI,
        fontSize: headSize,
        fontStyle: 'bold',
        color: UI_COLOR.accentSoft,
      });
      rightY += 22;
      this.add.text(rightX, rightY, thanks, {
        fontFamily: FONT_UI,
        fontSize: bodySize,
        color: UI_COLOR.muted,
        lineSpacing: lineSp + 2,
        wordWrap: { width: wrapWidth, useAdvancedWrap: true },
      });

      void leftY;
    } else {
      this.add.text(marginX, y, '免責事項 / Disclaimer', {
        fontFamily: FONT_UI,
        fontSize: headSize,
        fontStyle: 'bold',
        color: UI_COLOR.accentSoft,
      });
      y += 28;

      const dText = this.add.text(marginX, y, disclaimer, {
        fontFamily: FONT_UI,
        fontSize: bodySize,
        color: UI_COLOR.muted,
        lineSpacing: lineSp,
        wordWrap: { width: wrapWidth, useAdvancedWrap: true },
      });
      y += dText.height + 24;

      this.add.text(marginX, y, '技術スタック / Technology Stack', {
        fontFamily: FONT_UI,
        fontSize: headSize,
        fontStyle: 'bold',
        color: UI_COLOR.accentSoft,
      });
      y += 28;

      this.add.text(marginX, y, tech, {
        fontFamily: FONT_UI,
        fontSize: bodySize,
        color: UI_COLOR.muted,
        lineSpacing: 8,
        wordWrap: { width: wrapWidth, useAdvancedWrap: true },
      });
      y += 108;

      this.add.text(marginX, y, 'Special Thanks', {
        fontFamily: FONT_UI,
        fontSize: headSize,
        fontStyle: 'bold',
        color: UI_COLOR.accentSoft,
      });
      y += 28;

      this.add.text(marginX, y, thanks, {
        fontFamily: FONT_UI,
        fontSize: bodySize,
        color: UI_COLOR.muted,
        lineSpacing: 8,
        wordWrap: { width: wrapWidth, useAdvancedWrap: true },
      });
    }

    this.createHoverButton(width / 2, height - (landscapeCompact ? 40 : 52), '戻る（Back）', () => {
      this.scene.start('LoginScene');
    });
  }

  private createHoverButton(x: number, y: number, label: string, onClick: () => void) {
    const btn = this.add
      .text(x, y, label, {
        fontFamily: FONT_UI,
        fontSize: '16px',
        fontStyle: 'bold',
        color: UI_COLOR.ivory,
        backgroundColor: UI_COLOR.accent,
        padding: { x: 28, y: 12 },
      })
      .setOrigin(0.5)
      .setInteractive({ useHandCursor: true });

    btn.on('pointerover', () => btn.setStyle({ backgroundColor: '#f07060' }));
    btn.on('pointerout', () => btn.setStyle({ backgroundColor: UI_COLOR.accent }));
    btn.on('pointerdown', onClick);
    return btn;
  }
}
