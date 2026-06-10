# LLM Transformer

本文主要推演 Transformer 架构中的矩阵变换，并讨论 KV cache 的原理细节。

## 训练阶段

### 1. Encoder

Encoder 的输入矩阵: [I, love, NLP]

token: [batch_size, seq_len, emb_size]

**定义 W 矩阵:**

Wq: [emb_size, attention_size * multi_head]

Wk: [emb_size, attention_size * multi_head]

Wv: [emb_size, attention_size * multi_head]

**输出 Q/K/V:**

Q: [batch_size, seq_len, attention_size]

K: [batch_size, seq_len, attention_size]

V: [batch_size, seq_len, attention_size]

> 注：这里可以理解为单个 head 下的维度；如果展开多头，则通常会包含 `multi_head` 维度。

**由评分公式:**

softmax(Q * K^T / √d + padding_mask) * V  
= [seq_len, seq_len] * [seq_len, attention_size]

**输出 Encoder 结构:**

-> [batch_size, seq_len, attention_size]

-> FFN [batch_size, seq_len, emb_size]

### 2. Decoder

Decoder 的输入矩阵: [我, 爱, 自然, 语言, 处理]

Decoder 的过程，是通过 causal mask 进行掩码的。同一 seq 内，多个 token 可以批量处理。

以 `[我, 爱]` 推出 `自然` 为例，causal_mask 如下:

```text
(上三角矩阵)
[
  [0, -∞, -∞, -∞, -∞],
  [0,  0, -∞, -∞, -∞],
  [0,  0,  0, -∞, -∞],
  [0,  0,  0,  0, -∞],
  [0,  0,  0,  0,  0]
]
```

对 “我爱” 进行 `embedding + position encoding`

**当前参数值:**

target_seq_len = 5

causal_mask = [0, 0, -∞, -∞, -∞]

**当前 token:**

token: [batch_size, target_seq_len, emb_size]

#### 每个 Decoder 内部:

(1) 第一步：Masked Self-Attention

**定义 W 矩阵:**

Wq: [emb_size, attention_size * multi_head]

Wk: [emb_size, attention_size * multi_head]

Wv: [emb_size, attention_size * multi_head]

> causal_mask 不直接加在 W 矩阵上，而是加在 `QK^T` 的注意力分数上。

**输出 Q/K/V:**

==> Q, K, V（训练阶段不需要 K/V cache, 可以延伸思考推理阶段）==>

```text
Q: [target_seq_len, attention_size] -> 推理时仅需要计算最后一行 target_seq_len=2 时的 q
K: [target_seq_len, attention_size] -> 推理时仅需要计算最后一行 target_seq_len=2 时的 k，并进行缓存
V: [target_seq_len, attention_size] -> 推理时仅需要计算最后一行 target_seq_len=2 时的 v，并进行缓存
```

注意力计算:

softmax(Q * K^T / √d + causal_mask) * V


(2) 第二步：Cross-Attention

**定义 W 矩阵:**

使用交叉注意力机制 => 由 Encoder 输出的 `[batch_size, src_seq_len, emb_size]`

Wq: [emb_size, attention_size * multi_head]

> Encoder 输出不需要 causal_mask

Wk: [emb_size, attention_size * multi_head]

Wv: [emb_size, attention_size * multi_head]

**注意：QK^T = [target_seq_len, source_seq_len]**

==> 推出 QK^T = [target_seq_len, source_seq_len]，非常重要，代表 `目标序列` 对 `源序列` 的关注程度。

```text
              I     love    NLP
<BOS>        0.1    0.2    0.1
我           0.7    0.1    0.1
爱           0.1    0.8    0.2
```

**输出 Q/K/V:**

==> Q, K, V（训练阶段不需要 K/V cache, 可以延伸思考推理阶段）==>

```text
Q: [target_seq_len, attention_size] -> 推理时仅需要计算最后一行 seq_len=2 时的 q
K: [src_seq_len, attention_size]    -> 来自 Encoder 输出，可提前计算并缓存
V: [src_seq_len, attention_size]    -> 来自 Encoder 输出，可提前计算并缓存
```

最后经过 FFN（相对于 Q/K/V，计算参数量较大）

hidden_size → vocab_size

经过 softmax，得到每个 token 的概率，模型用采样策略选择下一个 token。

## 推理阶段

### 3. KV cache

主要讨论 Decoder-only 模型。

比如已经生成: [我, 爱]

token: [batch_size, seq_len, emb_size]

**W 矩阵值已固定:**

Wq、Wk、Wv 参数已经固定。

**输出 Q/K/V:**

==> Q, K, V（推理阶段的 K/V cache）==>

```text
Q: [seq_len, attention_size] -> 推理时仅需要计算最后一行 seq_len=2 时的 q
K: [seq_len, attention_size] -> 推理时仅需要计算最后一行 seq_len=2 时的 k，并进行缓存
V: [seq_len, attention_size] -> 推理时仅需要计算最后一行 seq_len=2 时的 v，并进行缓存
```

Q、K、V 的值会随着 seq_len 增大而逐步向下展开；其中，之前已经计算得到的 K、V 结果不会发生变化。

因此，可以通过对 K、V 进行有效缓存，消耗一定显存，减少重复计算，从而提高 GPU 推理效率。

## 图片流程

<center>
    <img src="https://github.com/yushaolong10/learning-docs/blob/master/images/2026/20260610-transformer_1.png">
</center>

<center>
    <img src="https://github.com/yushaolong10/learning-docs/blob/master/images/2026/20260610-transformer_2.png">
</center>

<center>
    <img src="https://github.com/yushaolong10/learning-docs/blob/master/images/2026/20260610-transformer_3.png">
</center>

<center>
    <img src="https://github.com/yushaolong10/learning-docs/blob/master/images/2026/20260610-transformer_4.png">
</center>
