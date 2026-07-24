import * as Phaser from 'phaser';
import { FONT_DISPLAY, FONT_UI, PHASER_COLOR, UI_COLOR } from '../constants/theme';

export class CreditScene extends Phaser.Scene {
  constructor() {
    super('CreditScene');
  }

  create() {
    const { width, height } = this.scale;
    const marginX = 52;
    const wrapWidth = width - marginX * 2;

    const panel = this.add.graphics();
    panel.fillStyle(PHASER_COLOR.panel, 0.82);
    panel.fillRoundedRect(28, 20, width - 56, height - 40, 18);
    panel.lineStyle(1, 0xe8eef4, 0.1);
    panel.strokeRoundedRect(28, 20, width - 56, height - 40, 18);

    let y = 44;

    const title = this.add.text(width / 2, y, 'Credits & Disclaimer', {
      fontFamily: FONT_DISPLAY,
      fontSize: '28px',
      color: UI_COLOR.ivory,
      align: 'center',
    });
    title.setOrigin(0.5, 0);
    y += 46;

    this.add.text(marginX, y, '免責事項 / Disclaimer', {
      fontFamily: FONT_UI,
      fontSize: '15px',
      fontStyle: 'bold',
      color: UI_COLOR.accentSoft,
    });
    y += 28;

    const disclaimer =
      '本作は二人麻雀「十七歩（じゅうななほ）」を題材とした非営利のファンメイド対戦デモです。\n' +
      'ゲームロジックとリアルタイム通信の学習・検証を目的としており、商業利用を行う予定はありません。\n' +
      '麻雀のルール・用語は伝統的な慣例および本プロジェクトの設計書に基づきます。';

    const dText = this.add.text(marginX, y, disclaimer, {
      fontFamily: FONT_UI,
      fontSize: '13px',
      color: UI_COLOR.muted,
      lineSpacing: 6,
      wordWrap: { width: wrapWidth, useAdvancedWrap: true },
    });
    y += dText.height + 24;

    this.add.text(marginX, y, '技術スタック / Technology Stack', {
      fontFamily: FONT_UI,
      fontSize: '15px',
      fontStyle: 'bold',
      color: UI_COLOR.accentSoft,
    });
    y += 28;

    this.add.text(
      marginX,
      y,
      '• Backend: Go, WebSocket（インメモリ・2人対戦）\n' +
        '• Frontend: Phaser 3, TypeScript, Vite\n' +
        '• Infra: Docker\n' +
        '• Development Assistant: AI（Cursor）— 人間との共同開発',
      {
        fontFamily: FONT_UI,
        fontSize: '13px',
        color: UI_COLOR.muted,
        lineSpacing: 8,
        wordWrap: { width: wrapWidth, useAdvancedWrap: true },
      }
    );
    y += 108;

    this.add.text(marginX, y, 'Special Thanks', {
      fontFamily: FONT_UI,
      fontSize: '15px',
      fontStyle: 'bold',
      color: UI_COLOR.accentSoft,
    });
    y += 28;

    this.add.text(
      marginX,
      y,
      '• 賭博堕天録カイジ — 原作者\n' +
        '• 麻雀王国 — 牌画像の出典元',
      {
        fontFamily: FONT_UI,
        fontSize: '13px',
        color: UI_COLOR.muted,
        lineSpacing: 8,
        wordWrap: { width: wrapWidth, useAdvancedWrap: true },
      }
    );

    this.createHoverButton(width / 2, height - 52, '戻る（Back）', () => {
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
