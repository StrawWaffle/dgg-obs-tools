import { describe, expect, it } from 'vitest';
import { readableColor } from './color';

describe('readable name colours', () => {
  it('leaves a colour that is already light enough', () => {
    expect(readableColor('#FF55B3')).toBe('#FF55B3');
    expect(readableColor('#53FC19')).toBe('#53FC19');
  });

  it('lightens black and near-black towards white', () => {
    expect(readableColor('#000000')).toBe('#595959');
    expect(readableColor('#0000ff')).not.toBe('#0000ff');
  });

  it('passes anything that is not a plain colour through', () => {
    expect(readableColor('red')).toBe('red');
  });
});
